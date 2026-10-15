package billing

import (
	"context"
	"time"
)

type UsageReader interface {
	GetUsage(ctx context.Context, customerID string, start, end time.Time) (inputTokens, outputTokens int64, err error)
}
