package billing

import (
	"context"
	"fmt"
	"strconv"

	"github.com/stripe/stripe-go/v86"
)

type StripeReporter struct {
	client *stripe.Client
}

func NewStripeReporter(apiKey string) (*StripeReporter, error) {
	sc := stripe.NewClient(apiKey)
	return &StripeReporter{
		client: sc,
	}, nil
}

func (sp *StripeReporter) RecordUsage(ctx context.Context, customerID string, inputTokens, outputTokens int64) error {

	events := []struct {
		eventName string
		value     int64
	}{
		{"anthropic_input_tokens", inputTokens},
		{"anthropic_output_tokens", outputTokens},
	}

	for _, e := range events {
		_, err := sp.client.V1BillingMeterEvents.Create(ctx, &stripe.BillingMeterEventCreateParams{
			EventName: stripe.String(e.eventName),
			Payload: map[string]string{
				"stripe_customer_id": customerID,
				"value":              strconv.FormatInt(e.value, 10),
			},
		})
		if err != nil {
			return fmt.Errorf("recording %s usage: %w", e.eventName, err)
		}
	}

	return nil
}
