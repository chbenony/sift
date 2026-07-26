package main

import (
	"log"
	"net/http"
)

type AnthropicRequest struct {
	Model     string    `json:"model,omitempty"`
	MaxTokens int64     `json:"max_tokens,omitempty"`
	Messages  []Message `json:"messages"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type Content struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type AnthropicResponse struct {
	Content    []Content `json:"content"`
	StopReason string    `json:"stop_reason"`
}

func ServeHttp(w http.ResponseWriter, r *http.Request) {
	_, err := w.Write([]byte(`hello world`))
	if err != nil {
		log.Printf("error writing response %v", err)
	}
}

func main() {
	http.Handle("/", http.HandlerFunc(ServeHttp))
	err := http.ListenAndServe(":9000", nil)
	if err != nil {
		log.Fatalf("error")
	}
}
