package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"sift/internal/anthropic"
	"sift/internal/auth"
	"sift/internal/billing"
	"sync"

	jwtmiddleware "github.com/auth0/go-jwt-middleware/v3"
	"github.com/auth0/go-jwt-middleware/v3/validator"
)

// maxBodyBytes is 1 MiB, I guess we'll adjust if needed
const maxBodyBytes = 1 << 20

func chatHandler(anthropicClient *anthropic.Client, reporter billing.Reporter, wg *sync.WaitGroup) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		wg.Add(1)
		defer wg.Done()

		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxBodyBytes))
		if err != nil {
			if _, ok := errors.AsType[*http.MaxBytesError](err); ok {
				http.Error(w, "request body is too large", http.StatusRequestEntityTooLarge)
				return
			}
			http.Error(w, "failed to read request body", http.StatusBadRequest)
			return
		}

		var reqData anthropic.Request
		if err := json.Unmarshal(body, &reqData); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		if reqData.Stream {
			http.Error(w, "streaming responses are not supported", http.StatusBadRequest)
			return
		}

		usrIdentity, err := jwtmiddleware.GetClaims[*validator.ValidatedClaims](r.Context())
		if err != nil {
			http.Error(w, "failed to get claims", http.StatusInternalServerError)
			return
		}
		claims, ok := usrIdentity.CustomClaims.(*auth.CustomClaims)
		if !ok {
			http.Error(w, "failed to get stripe customer id", http.StatusInternalServerError)
			return
		}

		result, err := anthropicClient.Send(r.Context(), body)
		if err != nil {
			http.Error(w, "upstream request failed", http.StatusBadGateway)
			return
		}
		if result.ParseErr != nil {
			log.Printf("failed to parse response body for logging: %v", result.ParseErr)
		}
		log.Printf("stop_reason=%s", result.Parsed.StopReason)

		w.Header().Set("content-type", "application/json")
		w.WriteHeader(result.StatusCode)
		if _, err := w.Write(result.Body); err != nil {
			log.Printf("error writing response to caller: %v", err)
		}

		if result.ParseErr == nil && result.StatusCode >= 200 && result.StatusCode < 300 && result.Parsed.ID != "" {
			wg.Go(func() {
				ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), billingTimeout)
				defer cancel()

				if err := reporter.RecordUsage(ctx, result.Parsed.ID, claims.StripeCustomerID,
					result.Parsed.Usage.InputTokens, result.Parsed.Usage.OutputTokens); err != nil {
					log.Printf("failed to get user's record usage: %v", err)
				}
			})
		}

	}
}
