package billing

import (
	"context"
	"fmt"
	"strconv"

	"github.com/stripe/stripe-go/v86"
)

type StripeReporter struct {
	client      *stripe.Client
	customerMap map[string]string
}

func NewStripeReporter(apiKey string, customerMap map[string]string) (*StripeReporter, error) {
	sc := stripe.NewClient(apiKey)
	return &StripeReporter{
		client:      sc,
		customerMap: customerMap,
	}, nil
}

func (sp *StripeReporter) RecordUsage(ctx context.Context, identity string, inputTokens, outputTokens int64) error {

	// this checks whether the identity of who is making the call maps to a
	// Stripe customerID
	customerID, ok := sp.customerMap[identity]
	if !ok {
		return fmt.Errorf("no stripe customer mapped for identity %q", identity)
	}

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
