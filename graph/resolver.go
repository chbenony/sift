package graph

import (
	"sift/internal/anthropic"
	"sift/internal/billing"
	"sync"
	"time"
)

// This file will not be regenerated automatically.
//
// It serves as dependency injection for your app, add any dependencies you require
// here.

// Stripe rejects start_time=0, so any minute-aligned time before the meters
// existed stands in for "all time"
var usageStart = time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)

const billingTimeout = 10 * time.Second

type Resolver struct {
	anthropicClient *anthropic.Client
	reporter        billing.Reporter
	usageReader     billing.UsageReader
	wg              *sync.WaitGroup
}

func NewResolver(client *anthropic.Client, reporter billing.Reporter, usageReader billing.UsageReader, wg *sync.WaitGroup) *Resolver {
	return &Resolver{
		anthropicClient: client,
		reporter:        reporter,
		usageReader:     usageReader,
		wg:              wg,
	}
}
