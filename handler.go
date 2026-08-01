package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"sift/internal/auth"
	"sift/internal/billing"
	"time"

	jwtmiddleware "github.com/auth0/go-jwt-middleware/v3"
	"github.com/auth0/go-jwt-middleware/v3/validator"
)

var (
	messagesURL = "https://api.anthropic.com/v1/messages"
)

// maxBodyBytes is 1 MiB, I guess we'll adjust if needed
const maxBodyBytes = 1 << 20

type AnthropicRequest struct {
	Model     string    `json:"model,omitempty"`
	MaxTokens int64     `json:"max_tokens,omitempty"`
	Messages  []Message `json:"messages"`
	Stream    bool      `json:"stream,omitempty"`
}

type Message struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

type Content struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type AnthropicResponse struct {
	ID         string    `json:"id"`
	Content    []Content `json:"content"`
	StopReason string    `json:"stop_reason"`
	Usage      Usage     `json:"usage"`
}

type Usage struct {
	InputTokens  int64 `json:"input_tokens"`
	OutputTokens int64 `json:"output_tokens"`
}

func chatHandler(apiKey string, client *http.Client, reporter billing.Reporter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if apiKey == "" {
			http.Error(w, "server is misconfigured", http.StatusInternalServerError)
			return
		}

		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxBodyBytes))
		if err != nil {
			if _, ok := errors.AsType[*http.MaxBytesError](err); ok {
				http.Error(w, "request body is too large", http.StatusRequestEntityTooLarge)
				return
			}
			http.Error(w, "failed to read request body", http.StatusBadRequest)
			return
		}

		var reqData AnthropicRequest
		if err := json.Unmarshal(body, &reqData); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		if reqData.Stream {
			http.Error(w, "streaming responses are not supported", http.StatusBadRequest)
			return
		}

		req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, messagesURL, bytes.NewReader(body))
		if err != nil {
			http.Error(w, "failed to build upstream request", http.StatusBadRequest)
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

		req.Header.Set("x-api-key", apiKey)
		req.Header.Set("anthropic-version", "2023-06-01")
		req.Header.Set("content-type", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			http.Error(w, "upstream request failed", http.StatusBadGateway)
			return
		}

		defer func() {
			if err := resp.Body.Close(); err != nil {
				log.Printf("error closing response body %v", err)
			}
		}()

		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			http.Error(w, "failed to read upstream response", http.StatusBadGateway)
			return
		}

		var anthropicResp AnthropicResponse
		unmarshalErr := json.Unmarshal(respBody, &anthropicResp)
		if unmarshalErr != nil {
			log.Printf("failed to parse response body for logging: %v", unmarshalErr)
		}
		log.Printf("stop_reason=%s", anthropicResp.StopReason)

		w.Header().Set("content-type", "application/json")
		w.WriteHeader(resp.StatusCode)
		if _, err := w.Write(respBody); err != nil {
			log.Printf("error writing response to caller: %v", err)
		}

		if unmarshalErr == nil && resp.StatusCode >= 200 && resp.StatusCode < 300 {
			go func() {
				ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), 10*time.Second)
				defer cancel()

				if err := reporter.RecordUsage(ctx, anthropicResp.ID, claims.StripeCustomerID,
					anthropicResp.Usage.InputTokens, anthropicResp.Usage.OutputTokens); err != nil {
					log.Printf("failed to get user's record usage: %v", err)
				}
			}()
		}

	}
}
