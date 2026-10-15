package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sift/graph"
	"sift/internal/anthropic"
	"sift/internal/auth"
	"sift/internal/billing"
	"sync"
	"syscall"
	"time"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/extension"
	"github.com/99designs/gqlgen/graphql/handler/lru"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	jwtmiddleware "github.com/auth0/go-jwt-middleware/v3"
	"github.com/vektah/gqlparser/v2/ast"
)

const (
	billingTimeout = 10 * time.Second
	// aliasing a field (e.g. a: usage{...} b: usage{...} ...) multiplies
	// resolver calls per request, so cap total query complexity.
	maxQueryComplexity = 100
)

func limitBody(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
		next.ServeHTTP(w, r)
	})
}

func main() {
	// Auth0 env variables
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	domain := os.Getenv("AUTH0_DOMAIN")
	audience := os.Getenv("AUTH0_AUDIENCE")

	// Stripe env variables
	stripeApiKey := os.Getenv("STRIPE_API_KEY")
	stripeTokensInputMeterId := os.Getenv("STRIPE_INPUT_TOKENS_METER_ID")
	stripeTokensOutputMeterId := os.Getenv("STRIPE_OUTPUT_TOKENS_METER_ID")

	//wait group
	var wg sync.WaitGroup

	timeout := 60 * time.Second
	if v := os.Getenv("ANTHROPIC_CLIENT_TIMEOUT"); v != "" {
		if parsed, err := time.ParseDuration(v); err == nil {
			timeout = parsed
		}
	}
	client := &http.Client{Timeout: timeout}
	anthropicClient, err := anthropic.NewClient(apiKey, client)
	if err != nil {
		log.Fatalf("failed to create new anthropic client: %v", err)
	}

	jwtValidator, err := auth.NewValidator(domain, audience)
	if err != nil {
		log.Fatalf("failed to create validator: %v", err)
	}

	reporter, err := billing.NewStripeReporter(stripeApiKey, stripeTokensInputMeterId, stripeTokensOutputMeterId)
	if err != nil {
		log.Fatalf("failed to create new stripe reporter: %v", err)
	}

	middleware, err := jwtmiddleware.New(jwtmiddleware.WithValidator(jwtValidator))
	if err != nil {
		log.Fatalf("failed to create middleware: %v", err)
	}

	resolver := graph.NewResolver(anthropicClient, reporter, reporter, &wg)
	graphqlHandler := handler.New(graph.NewExecutableSchema(graph.Config{
		Resolvers: resolver,
	}))
	// Built explicitly instead of handler.NewDefaultServer: this schema has no
	// subscriptions, and CheckJWT only validates the initial HTTP upgrade, so a
	// WebSocket transport would let a connection outlive its JWT.
	graphqlHandler.AddTransport(transport.Options{})
	graphqlHandler.AddTransport(transport.GET{})
	graphqlHandler.AddTransport(transport.POST{})
	graphqlHandler.AddTransport(transport.MultipartForm{})
	graphqlHandler.SetQueryCache(lru.New[*ast.QueryDocument](1000))
	graphqlHandler.Use(extension.Introspection{})
	graphqlHandler.Use(extension.AutomaticPersistedQuery{Cache: lru.New[string](100)})
	graphqlHandler.Use(extension.FixedComplexityLimit(maxQueryComplexity))
	http.Handle("/graphql", middleware.CheckJWT(limitBody(graphqlHandler)))

	http.Handle("/", middleware.CheckJWT(http.HandlerFunc(chatHandler(anthropicClient, reporter, &wg))))
	// TODO(hardening): no ReadHeaderTimeout/ReadTimeout configured, so a slow client can
	// hold a connection open indefinitely. Deferred until this is past scaffolding.
	srv := &http.Server{Addr: ":9000"}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	serveErr := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErr <- err
		}
	}()

	select {
	case err := <-serveErr:
		log.Fatalf("server error: %v", err)
	case <-ctx.Done():
		// proceed with graceful shutdown
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Printf("shutdown error: %v", err)
		}
	}

	waitDone := make(chan struct{})
	go func() {
		wg.Wait()
		close(waitDone)
	}()

	select {
	case <-waitDone:
		log.Println("all in-flight work completed")
	case <-time.After(billingTimeout):
		log.Println("timed out waiting for in-flight work; some usage may not have been recorded")
	}
}
