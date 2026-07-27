package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"time"
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
	Content    []Content `json:"content"`
	StopReason string    `json:"stop_reason"`
}

func main() {
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	client := &http.Client{Timeout: time.Second * 10}

	http.Handle("/", http.HandlerFunc(chatHandler(apiKey, client)))
	err := http.ListenAndServe(":9000", nil)
	if err != nil {
		log.Fatalf("error")
	}
}

func chatHandler(apiKey string, client *http.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if apiKey == "" {
			http.Error(w, "server is misconfigured", http.StatusInternalServerError)
			return
		}

		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxBodyBytes))
		if err != nil {
			var maxBytesErr *http.MaxBytesError
			if errors.As(err, &maxBytesErr) {
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

		outBody, err := json.Marshal(reqData)
		if err != nil {
			http.Error(w, "failed to build upstream request", http.StatusInternalServerError)
			return
		}

		req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, messagesURL, bytes.NewReader(outBody))
		if err != nil {
			http.Error(w, "failed to build upstream request", http.StatusBadRequest)
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
		if err := json.Unmarshal(respBody, &anthropicResp); err != nil {
			log.Printf("failed to parse response body for logging: %v", err)
		}
		log.Printf("stop_reason=%s", anthropicResp.StopReason)

		w.Header().Set("content-type", "application/json")
		w.WriteHeader(resp.StatusCode)
		if _, err := w.Write(respBody); err != nil {
			log.Printf("error writing response to caller: %v", err)
		}

	}
}
