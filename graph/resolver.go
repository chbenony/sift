package graph

import (
	"sift/internal/anthropic"
	"sift/internal/billing"
	"sync"
)

// This file will not be regenerated automatically.
//
// It serves as dependency injection for your app, add any dependencies you require
// here.

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
