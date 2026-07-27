package main

import (
	"log"
	"net/http"
	"os"
	"sift/internal/auth"
	"sift/internal/billing"
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

	http.Handle("/", middleware.CheckJWT(http.HandlerFunc(chatHandler(apiKey, client, reporter))))
	// TODO(hardening): no ReadHeaderTimeout/ReadTimeout configured, so a slow client can
	// hold a connection open indefinitely. Deferred until this is past scaffolding.
	err = http.ListenAndServe(":9000", nil)
	if err != nil {
		log.Fatalf("error")
	}
}
