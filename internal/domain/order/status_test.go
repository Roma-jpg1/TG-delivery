package order

import "testing"

func TestValidateTransitionManualReview(t *testing.T) {
	if err := ValidateTransition(StatusManualReview, StatusConfirmed); err != nil {
		t.Fatalf("expected manual_review -> confirmed to be allowed, got %v", err)
	}
	if err := ValidateTransition(StatusManualReview, StatusPreparing); err == nil {
		t.Fatalf("expected manual_review -> preparing to be disallowed")
	}
}
