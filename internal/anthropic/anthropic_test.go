package anthropic

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func Test_Anthropic(t *testing.T) {
	tests := []struct {
		name           string
		responseStatus int
		responseBody   string
		wantParseErr   bool
	}{
		{
			name:           "successful response",
			responseStatus: 200,
			responseBody: `{
				"id":"msg_1",
				"stop_reason":"end_turn",
				"usage":{
					"input_tokens": 1,
					"output_tokens": 1
				}
			}`,
			wantParseErr: false,
		},
		{
			name:           "malformed json",
			responseStatus: 200,
			responseBody:   `not json`,
			wantParseErr:   true,
		},
		{
			name:           "anthropic error response",
			responseStatus: 400,
			responseBody: `{
				"type":"error",
				"error":{
					"type": "invalid_request_error"
				}
			}`,
			wantParseErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.responseStatus)
				_, _ = w.Write([]byte(tt.responseBody))
			}))
			defer server.Close()

			client := &Client{
				apiKey:     "dummy-api-key",
				baseURL:    server.URL,
				httpClient: &http.Client{},
			}

			result, err := client.Send(context.Background(), []byte("{}"))
			if err != nil {
				t.Errorf("ParseErr = %v, wantParseErr = %v", result.ParseErr, tt.wantParseErr)
			}

			gotParseErr := result.ParseErr != nil
			if gotParseErr != tt.wantParseErr {
				t.Errorf("ParseErr = %v, wantParseErr = %v", result.ParseErr, tt.wantParseErr)
			}

		})
	}
}
