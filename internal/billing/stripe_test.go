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
	mockCreator.EXPECT().Create(gomock.Any(), gomock.Any()).
		Return(&stripe.BillingMeterEvent{}, nil).Times(2)

	reporter := &StripeReporter{meterEvents: mockCreator}
	err := reporter.RecordUsage(context.Background(), "msg_1", "cus_1", 10, 5)
	if err != nil {
		t.Errorf("RecordUsage() error = %v, want nil", err)
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
