package main

import (
	"log"
	"net/http"
	"os"
	"time"
)

func main() {
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	client := &http.Client{Timeout: time.Second * 10}

	// TODO(auth): no inbound authentication yet — any client that can reach this port
	// can spend the configured Anthropic API key. Tracked for the identity/policy milestone.
	http.Handle("/", http.HandlerFunc(chatHandler(apiKey, client)))
	// TODO(hardening): no ReadHeaderTimeout/ReadTimeout configured, so a slow client can
	// hold a connection open indefinitely. Deferred until this is past scaffolding.
	err := http.ListenAndServe(":9000", nil)
	if err != nil {
		log.Fatalf("error")
	}
}
