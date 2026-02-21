package payment

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/hugovillarreal/ridesharing/internal/models"
)

// --- Mock Repository ---

type mockPaymentRepo struct {
	payments  map[uuid.UUID]*models.Payment
	createErr error
}

func newMockPaymentRepo() *mockPaymentRepo {
	return &mockPaymentRepo{
		payments: make(map[uuid.UUID]*models.Payment),
	}
}

func (m *mockPaymentRepo) Create(ctx context.Context, payment *models.Payment) error {
	if m.createErr != nil {
		return m.createErr
	}
	payment.ID = uuid.New()
	payment.CreatedAt = time.Now()
	m.payments[payment.RideID] = payment
	return nil
}

func (m *mockPaymentRepo) GetByRideID(ctx context.Context, rideID uuid.UUID) (*models.Payment, error) {
	p, ok := m.payments[rideID]
	if !ok {
		return nil, nil
	}
	return p, nil
}

func (m *mockPaymentRepo) UpdateStatus(ctx context.Context, id uuid.UUID, status string) error {
	for _, p := range m.payments {
		if p.ID == id {
			p.Status = status
			return nil
		}
	}
	return nil
}

// --- Mock Payment Gateway ---

type mockGateway struct {
	txnID      string
	createErr  error
	captureErr error
}

func (m *mockGateway) CreatePaymentIntent(amount float64, currency string) (string, error) {
	if m.createErr != nil {
		return "", m.createErr
	}
	return m.txnID, nil
}

func (m *mockGateway) CapturePayment(txnID string) error {
	return m.captureErr
}

func (m *mockGateway) Refund(txnID string, amount float64) (string, error) {
	return "re_test", nil
}

// --- Tests ---

func TestChargeRide_Success(t *testing.T) {
	repo := newMockPaymentRepo()
	gw := &mockGateway{txnID: "pi_test_success"}
	svc := &Service{repo: repo, stripe: gw}

	rideID := uuid.New()
	payment, err := svc.ChargeRide(context.Background(), rideID, 25.50)
	if err != nil {
		t.Fatalf("ChargeRide failed: %v", err)
	}

	if payment.Amount != 25.50 {
		t.Errorf("expected amount 25.50, got %f", payment.Amount)
	}
	if payment.Currency != "USD" {
		t.Errorf("expected currency USD, got %q", payment.Currency)
	}
	if payment.Status != "completed" {
		t.Errorf("expected status completed, got %q", payment.Status)
	}
	if payment.StripeTxnID != "pi_test_success" {
		t.Errorf("expected txn ID pi_test_success, got %q", payment.StripeTxnID)
	}
	if payment.RideID != rideID {
		t.Errorf("expected ride ID %s, got %s", rideID, payment.RideID)
	}
}

func TestChargeRide_Idempotent(t *testing.T) {
	rideID := uuid.New()
	repo := newMockPaymentRepo()

	// Pre-populate a completed payment
	existingPayment := &models.Payment{
		ID:          uuid.New(),
		RideID:      rideID,
		Amount:      30.00,
		Currency:    "USD",
		Status:      "completed",
		StripeTxnID: "pi_existing",
		CreatedAt:   time.Now(),
	}
	repo.payments[rideID] = existingPayment

	gw := &mockGateway{txnID: "pi_should_not_be_used"}
	svc := &Service{repo: repo, stripe: gw}

	// Charge again — should return existing payment
	payment, err := svc.ChargeRide(context.Background(), rideID, 30.00)
	if err != nil {
		t.Fatalf("ChargeRide idempotent failed: %v", err)
	}

	if payment.StripeTxnID != "pi_existing" {
		t.Errorf("expected existing txn ID pi_existing, got %q (gateway was called again!)", payment.StripeTxnID)
	}
	if payment.ID != existingPayment.ID {
		t.Error("expected same payment object, got different ID")
	}
}

func TestChargeRide_GatewayFailure(t *testing.T) {
	repo := newMockPaymentRepo()
	gw := &mockGateway{createErr: errors.New("card declined")}
	svc := &Service{repo: repo, stripe: gw}

	_, err := svc.ChargeRide(context.Background(), uuid.New(), 25.00)
	if err == nil {
		t.Fatal("expected error when gateway fails")
	}
}

func TestChargeRide_CaptureFailure(t *testing.T) {
	repo := newMockPaymentRepo()
	gw := &mockGateway{
		txnID:      "pi_test",
		captureErr: errors.New("capture failed"),
	}
	svc := &Service{repo: repo, stripe: gw}

	_, err := svc.ChargeRide(context.Background(), uuid.New(), 25.00)
	if err == nil {
		t.Fatal("expected error when capture fails")
	}
}

func TestChargeRide_RepoFailure(t *testing.T) {
	repo := newMockPaymentRepo()
	repo.createErr = errors.New("db error")
	gw := &mockGateway{txnID: "pi_test"}
	svc := &Service{repo: repo, stripe: gw}

	_, err := svc.ChargeRide(context.Background(), uuid.New(), 25.00)
	if err == nil {
		t.Fatal("expected error when repo fails")
	}
}

func TestGetPayment_Found(t *testing.T) {
	rideID := uuid.New()
	repo := newMockPaymentRepo()
	repo.payments[rideID] = &models.Payment{
		ID:          uuid.New(),
		RideID:      rideID,
		Amount:      15.00,
		Currency:    "USD",
		Status:      "completed",
		StripeTxnID: "pi_found",
		CreatedAt:   time.Now(),
	}
	svc := &Service{repo: repo, stripe: &mockGateway{}}

	payment, err := svc.GetPayment(context.Background(), rideID)
	if err != nil {
		t.Fatalf("GetPayment failed: %v", err)
	}
	if payment.Amount != 15.00 {
		t.Errorf("expected amount 15.00, got %f", payment.Amount)
	}
}

func TestGetPayment_NotFound(t *testing.T) {
	repo := newMockPaymentRepo()
	svc := &Service{repo: repo, stripe: &mockGateway{}}

	_, err := svc.GetPayment(context.Background(), uuid.New())
	if err == nil {
		t.Fatal("expected error for missing payment")
	}
}
