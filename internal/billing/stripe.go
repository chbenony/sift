//go:generate mockgen -source=stripe.go -destination=stripe_mock_test.go -package=billing
package billing

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/stripe/stripe-go/v86"
)

type meterEventCreator interface {
	Create(ctx context.Context, params *stripe.BillingMeterEventCreateParams) (*stripe.BillingMeterEvent, error)
}

type StripeReporter struct {
	meterEvents meterEventCreator
}

func NewStripeReporter(apiKey string) (*StripeReporter, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("stripe api key must not be empty")
	}

	sc := stripe.NewClient(apiKey)
	return &StripeReporter{
		meterEvents: sc.V1BillingMeterEvents,
	}, nil
}

func (sp *StripeReporter) RecordUsage(ctx context.Context, messageID string, customerID string, inputTokens, outputTokens int64) error {

	events := []struct {
		eventName string
		value     int64
	}{
		{"anthropic_input_tokens", inputTokens},
		{"anthropic_output_tokens", outputTokens},
	}

	var recordErr error

	for _, e := range events {
		_, err := sp.meterEvents.Create(ctx, &stripe.BillingMeterEventCreateParams{
			Identifier: stripe.String(messageID + "-" + e.eventName),
			EventName:  stripe.String(e.eventName),
			Payload: map[string]string{
				"stripe_customer_id": customerID,
				"value":              strconv.FormatInt(e.value, 10),
			},
		})
		if err != nil {
			recordErr = errors.Join(recordErr, err)
		}
	}

	return recordErr
}
