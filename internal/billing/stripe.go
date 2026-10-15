//go:generate mockgen -source=stripe.go -destination=stripe_mock_test.go -package=billing
package billing

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/stripe/stripe-go/v86"
)

type meterEventCreator interface {
	Create(ctx context.Context, params *stripe.BillingMeterEventCreateParams) (*stripe.BillingMeterEvent, error)
}

type meterEventSummaryLister interface {
	List(ctx context.Context, params *stripe.BillingMeterEventSummaryListParams) stripe.Seq2[*stripe.BillingMeterEventSummary, error]
}

type summaryLister struct {
	client *stripe.Client
}

type StripeReporter struct {
	meterEvents         meterEventCreator
	meterEventSummaries meterEventSummaryLister
	inputMeterID        string
	outputMeterID       string
}

func (l summaryLister) List(ctx context.Context, params *stripe.BillingMeterEventSummaryListParams) stripe.Seq2[*stripe.BillingMeterEventSummary, error] {
	return l.client.V1BillingMeterEventSummaries.List(ctx, params).All(ctx)
}

func NewStripeReporter(apiKey, inputMeterID, outputMeterID string) (*StripeReporter, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("stripe api key must not be empty")
	}

	sc := stripe.NewClient(apiKey)
	return &StripeReporter{
		meterEvents:         sc.V1BillingMeterEvents,
		meterEventSummaries: summaryLister{client: sc},
		inputMeterID:        inputMeterID,
		outputMeterID:       outputMeterID,
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

func (sp *StripeReporter) GetUsage(ctx context.Context, customerID string, start, end time.Time) (inputTokens, outputTokens int64, err error) {

	inputTokens, err = sp.getMeterUsage(ctx, sp.inputMeterID, customerID, start, end)
	if err != nil {
		return 0, 0, fmt.Errorf("getting input token usage: %w", err)
	}

	outputTokens, err = sp.getMeterUsage(ctx, sp.outputMeterID, customerID, start, end)
	if err != nil {
		return 0, 0, fmt.Errorf("getting output token usage: %w", err)
	}

	return inputTokens, outputTokens, nil
}

// maxSummaryWindow is Stripe's maximum range for a single meter event
// summary query.
const maxSummaryWindow = 35 * 24 * time.Hour

func (sp *StripeReporter) getMeterUsage(ctx context.Context, meterID, customerID string, start, end time.Time) (int64, error) {

	var total float64
	for windowStart := start; windowStart.Before(end); windowStart = windowStart.Add(maxSummaryWindow) {
		windowEnd := windowStart.Add(maxSummaryWindow)
		if windowEnd.After(end) {
			windowEnd = end
		}

		for summary, err := range sp.meterEventSummaries.List(ctx, &stripe.BillingMeterEventSummaryListParams{
			ID:        stripe.String(meterID),
			Customer:  stripe.String(customerID),
			StartTime: stripe.Int64(windowStart.Unix()),
			EndTime:   stripe.Int64(windowEnd.Unix()),
		}) {
			if err != nil {
				return 0, err
			}
			total += summary.AggregatedValue
			//no ValueGroupingWindow set, so exactly one summary is expected per window
			break
		}
	}
	return int64(total), nil
}
