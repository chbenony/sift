package billing

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

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

func Test_GetUsage_Success(t *testing.T) {
	ctrl := gomock.NewController(t)

	mockLister := NewMockmeterEventSummaryLister(ctrl)

	seq := func(v float64) stripe.Seq2[*stripe.BillingMeterEventSummary, error] {
		return func(yield func(*stripe.BillingMeterEventSummary, error) bool) {
			yield(&stripe.BillingMeterEventSummary{AggregatedValue: v}, nil)
		}
	}

	var gotParams []*stripe.BillingMeterEventSummaryListParams

	//one expectation per meter, in order: input meter first, then output meter
	gomock.InOrder(
		mockLister.EXPECT().List(gomock.Any(), gomock.Any()).DoAndReturn(
			func(ctx context.Context, params *stripe.BillingMeterEventSummaryListParams) stripe.Seq2[*stripe.BillingMeterEventSummary, error] {
				gotParams = append(gotParams, params)
				return (seq(100))
			},
		),
		mockLister.EXPECT().List(gomock.Any(), gomock.Any()).DoAndReturn(
			func(ctx context.Context, params *stripe.BillingMeterEventSummaryListParams) stripe.Seq2[*stripe.BillingMeterEventSummary, error] {
				gotParams = append(gotParams, params)
				return (seq(40))
			},
		),
	)

	reporter := &StripeReporter{
		meterEventSummaries: mockLister,
		inputMeterID:        "mtr_in",
		outputMeterID:       "mtr_out",
	}

	actualInput, actualOutput, err := reporter.GetUsage(context.Background(), "cus_1",
		time.Now().Add(-1*time.Hour), time.Now())

	if err != nil {
		t.Fatalf("GetUsage() error = %v, want nil", err)
	}

	if actualInput != 100 {
		t.Errorf("actualInput = %v, want 100", actualInput)
	}

	if actualOutput != 40 {
		t.Errorf("actualOutput = %v, want 40", actualOutput)
	}

	if len(gotParams) != 2 {
		t.Fatalf("got %d List calls, want 2", len(gotParams))
	}

	if *gotParams[0].ID != "mtr_in" {
		t.Errorf("gotParams[0].ID = %v, want mtr_in", *gotParams[0].ID)
	}

	if *gotParams[0].Customer != "cus_1" {
		t.Errorf("gotParams[0].Customer = %v, want cus_1", *gotParams[0].Customer)
	}

	if *gotParams[1].ID != "mtr_out" {
		t.Errorf("gotParams[1].ID = %v, want mtr_out", *gotParams[1].ID)
	}
}

func Test_GetUsage_InputMeterFails(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockLister := NewMockmeterEventSummaryLister(ctrl)

	mockLister.EXPECT().List(gomock.Any(), gomock.Any()).Return(
		summarySeq(0, errors.New("stripe down")),
	)

	reporter := &StripeReporter{
		meterEventSummaries: mockLister,
		inputMeterID:        "mtr_in",
		outputMeterID:       "mtr_out",
	}

	gotInput, gotOutput, err := reporter.GetUsage(context.Background(), "cus_1", time.Now().Add(-2*time.Minute), time.Now())
	if err == nil {
		t.Error("GetUsage() error = nil, want non-nil")
	}

	if !strings.Contains(err.Error(), "input") {
		t.Errorf("GetUsage() error = %v, want it to mention input meter", err)
	}

	if gotInput != 0 || gotOutput != 0 {
		t.Errorf("GetUsage() = (%d, %d), want (0, 0) on failure", gotInput, gotOutput)
	}

}

func Test_GetUsage_OutputMeterFails(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockLister := NewMockmeterEventSummaryLister(ctrl)

	gomock.InOrder(
		mockLister.EXPECT().List(gomock.Any(), gomock.Any()).Return(summarySeq(100, nil)),
		mockLister.EXPECT().List(gomock.Any(), gomock.Any()).Return(summarySeq(0, errors.New("stripe down"))),
	)

	reporter := &StripeReporter{
		meterEventSummaries: mockLister,
		inputMeterID:        "mtr_in",
		outputMeterID:       "mtr_out",
	}

	gotInput, gotOutput, err := reporter.GetUsage(context.Background(), "cus_1", time.Now().Add(-2*time.Minute), time.Now())
	if err == nil {
		t.Error("GetUsage() error = nil, want non-nil")
	}

	if !strings.Contains(err.Error(), "output") {
		t.Errorf("GetUsage() error = %v, want it to mention output meter", err)
	}

	if gotInput != 0 || gotOutput != 0 {
		t.Errorf("GetUsage() = (%d, %d), want (0, 0) on failure", gotInput, gotOutput)
	}

}

func summarySeq(value float64, err error) stripe.Seq2[*stripe.BillingMeterEventSummary, error] {
	return func(yield func(*stripe.BillingMeterEventSummary, error) bool) {
		if err != nil {
			yield(nil, err)
			return
		}
		yield(&stripe.BillingMeterEventSummary{AggregatedValue: value}, nil)
	}
}
