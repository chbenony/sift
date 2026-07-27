package billing

import "context"

type Reporter interface {
	RecordUsage(ctx context.Context, customerID string, inputTokens, outputTokens int64) error
}
