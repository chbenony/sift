package auth

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

func Test_CustomClaims_Validate(t *testing.T) {
	ctx := context.Background()
	cusId := uuid.NewString()
	valid := &CustomClaims{StripeCustomerID: cusId}
	if err := valid.Validate(ctx); err != nil {
		t.Errorf("Validate() error = %v, want nil", err)
	}

	missing := &CustomClaims{StripeCustomerID: ""}
	if err := missing.Validate(ctx); err == nil {
		t.Error("Validate() error = nil, want non-nil")
	}
}
