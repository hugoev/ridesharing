package payment

import (
	"testing"
)

func TestStripeStub_CreatePaymentIntent(t *testing.T) {
	stub := &StripeStub{}

	// Run multiple times to test both success and potential simulated failure paths
	successCount := 0
	attempts := 100

	for i := 0; i < attempts; i++ {
		txnID, err := stub.CreatePaymentIntent(25.00, "USD")
		if err != nil {
			// Simulated failures are expected (2% rate)
			continue
		}
		successCount++

		if txnID == "" {
			t.Error("expected non-empty transaction ID on success")
		}

		// Verify transaction ID format
		if len(txnID) < 10 {
			t.Errorf("txnID %q too short", txnID)
		}
	}

	// Should succeed most of the time (98%)
	if successCount < attempts*90/100 {
		t.Errorf("success rate too low: %d/%d", successCount, attempts)
	}
}

func TestStripeStub_CapturePayment(t *testing.T) {
	stub := &StripeStub{}

	err := stub.CapturePayment("pi_test_12345")
	if err != nil {
		t.Errorf("CapturePayment failed: %v", err)
	}
}

func TestStripeStub_Refund(t *testing.T) {
	stub := &StripeStub{}

	refundID, err := stub.Refund("pi_test_12345", 10.00)
	if err != nil {
		t.Errorf("Refund failed: %v", err)
	}
	if refundID == "" {
		t.Error("expected non-empty refund ID")
	}
}

func TestStripeStub_UniqueTransactionIDs(t *testing.T) {
	stub := &StripeStub{}
	seen := make(map[string]bool)

	for i := 0; i < 20; i++ {
		txnID, err := stub.CreatePaymentIntent(10.00, "USD")
		if err != nil {
			continue // skip simulated failures
		}
		if seen[txnID] {
			t.Errorf("duplicate transaction ID: %s", txnID)
		}
		seen[txnID] = true
	}
}
