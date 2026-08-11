package billing

import (
	"context"
	"errors"
	"testing"

	"github.com/stripe/stripe-go/v86"
	"go.uber.org/mock/gomock"
)

func Test_RecordUsage_BothEventsSucceed(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockCreator := NewMockmeterEventCreator(ctrl)

	var gotParams []*stripe.BillingMeterEventCreateParams

	mockCreator.EXPECT().Create(gomock.Any(), gomock.Any()).
		DoAndReturn(
			func(ctx context.Context, params *stripe.BillingMeterEventCreateParams) (*stripe.BillingMeterEvent, error) {
				gotParams = append(gotParams, params)
				return &stripe.BillingMeterEvent{}, nil
			},
		).Times(2)

	reporter := &StripeReporter{meterEvents: mockCreator}
	err := reporter.RecordUsage(context.Background(), "msg_1", "cus_1", 10, 5)
	if err != nil {
		t.Errorf("RecordUsage() error = %v, want nil", err)
	}

	if len(gotParams) != 2 {
		t.Fatalf("got %d Create calls, want 2", len(gotParams))
	}

	if *gotParams[0].EventName != "anthropic_input_tokens" {
		t.Errorf("gotParams[0].EventName = %v, want anthropic_input_tokens", *gotParams[0].EventName)
	}

	if *gotParams[0].Identifier != "msg_1-anthropic_input_tokens" {
		t.Errorf("gotParams[0].Identifier = %v, want msg_1-anthropic_input_tokens", *gotParams[0].Identifier)
	}

	if gotParams[0].Payload["value"] != "10" {
		t.Errorf("gotParams[0] value = %v, want 10", gotParams[0].Payload)
	}

	if *gotParams[1].EventName != "anthropic_output_tokens" {
		t.Errorf("gotParams[1].EventName = %v, want anthropic_output_tokens", *gotParams[1].EventName)
	}

	if *gotParams[1].Identifier != "msg_1-anthropic_output_tokens" {
		t.Errorf("gotParams[1].Identifier = %v, want msg_1-anthropic_output_tokens", *gotParams[1].Identifier)
	}

	if gotParams[1].Payload["value"] != "5" {
		t.Errorf("gotParams[1] value = %v, want 5", gotParams[1].Payload)
	}
}

func Test_RecordUsage_FirstEventFailsSecondStillAttempted(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockCreator := NewMockmeterEventCreator(ctrl)

	gomock.InOrder(
		mockCreator.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil, errors.New("stripe down")),
		mockCreator.EXPECT().Create(gomock.Any(), gomock.Any()).Return(&stripe.BillingMeterEvent{}, nil),
	)

	reporter := &StripeReporter{meterEvents: mockCreator}
	err := reporter.RecordUsage(context.Background(), "msg_1", "cus_1", 10, 5)

	if err == nil {
		t.Error("RecordUsage() error = nil, want non-nil")
	}
}
