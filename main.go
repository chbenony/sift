package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sift/internal/auth"
	"sift/internal/billing"
	"sync"
	"syscall"
	"time"

	jwtmiddleware "github.com/auth0/go-jwt-middleware/v3"
)

func main() {
	// Auth0 env variables
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	domain := os.Getenv("AUTH0_DOMAIN")
	audience := os.Getenv("AUTH0_AUDIENCE")

	// Stripe env variables
	stripeApiKey := os.Getenv("STRIPE_API_KEY")

	//wait group
	var wg sync.WaitGroup

	timeout := 60 * time.Second
	if v := os.Getenv("ANTHROPIC_CLIENT_TIMEOUT"); v != "" {
		if parsed, err := time.ParseDuration(v); err == nil {
			timeout = parsed
		}
	}
	client := &http.Client{Timeout: timeout}

	jwtValidator, err := auth.NewValidator(domain, audience)
	if err != nil {
		log.Fatalf("failed to create validator: %v", err)
	}

	reporter, err := billing.NewStripeReporter(stripeApiKey)
	if err != nil {
		log.Fatalf("failed to create new stripe reporter: %v", err)
	}

	middleware, err := jwtmiddleware.New(jwtmiddleware.WithValidator(jwtValidator))
	if err != nil {
		log.Fatalf("failed to create middleware: %v", err)
	}

	http.Handle("/", middleware.CheckJWT(http.HandlerFunc(chatHandler(apiKey, client, reporter, &wg))))
	// TODO(hardening): no ReadHeaderTimeout/ReadTimeout configured, so a slow client can
	// hold a connection open indefinitely. Deferred until this is past scaffolding.
	srv := &http.Server{Addr: ":9000"}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("server error: %v", err)
		}
	}()
	<-ctx.Done() //main blocks here until SIGINT/SIGTERM

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown error: %v", err)
	}
	wg.Wait()
}
