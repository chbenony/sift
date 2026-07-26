package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"
)

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

	}
}
