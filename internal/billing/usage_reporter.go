package billing

import "context"

type Reporter interface {
	RecordUsage(ctx context.Context, identity string, inputTokens, outputTokens int64) error
}
