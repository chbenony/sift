package anthropic

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
)

var (
	messagesURL = "https://api.anthropic.com/v1/messages"
)

type Request struct {
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

type Response struct {
	ID         string    `json:"id"`
	Content    []Content `json:"content"`
	StopReason string    `json:"stop_reason"`
	Usage      Usage     `json:"usage"`
}

type Usage struct {
	InputTokens  int64 `json:"input_tokens"`
	OutputTokens int64 `json:"output_tokens"`
}

type Client struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
}

type Result struct {
	Body       []byte
	Parsed     *Response
	StatusCode int
	ParseErr   error // this is non-nil if the body wasn't valid or expected JSON
}

func NewClient(apiKey string, httpClient *http.Client) (*Client, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("anthropic api key must not be empty")
	}
	if httpClient == nil {
		return nil, fmt.Errorf("httpClient must not be nil")
	}
	return &Client{
		apiKey:     apiKey,
		baseURL:    messagesURL,
		httpClient: httpClient,
	}, nil
}

func (c *Client) Send(ctx context.Context, body []byte) (*Result, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("content-type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Printf("error closing response body %v", err)
		}
	}()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var anthropicResp Response
	unmarshalErr := json.Unmarshal(respBody, &anthropicResp)
	if unmarshalErr != nil {
		log.Printf("failed to parse response body for logging: %v", unmarshalErr)
	}

	return &Result{
		Body:       respBody,
		Parsed:     &anthropicResp,
		StatusCode: resp.StatusCode,
		ParseErr:   unmarshalErr,
	}, nil
}
