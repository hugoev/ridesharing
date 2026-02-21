// Package payment implements the payment service with a Stripe stub.
package payment

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"

	"github.com/hugovillarreal/ridesharing/internal/models"
	"github.com/hugovillarreal/ridesharing/pkg/db"
	apperrors "github.com/hugovillarreal/ridesharing/pkg/errors"
	"github.com/hugovillarreal/ridesharing/pkg/tracing"
)

// PaymentGateway abstracts the external payment provider.
type PaymentGateway interface {
	CreatePaymentIntent(amount float64, currency string) (string, error)
	CapturePayment(txnID string) error
	Refund(txnID string, amount float64) (string, error)
}

// RepositoryInterface defines the contract for payment data access.
type RepositoryInterface interface {
	Create(ctx context.Context, payment *models.Payment) error
	GetByRideID(ctx context.Context, rideID uuid.UUID) (*models.Payment, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status string) error
}

// StripeStub simulates Stripe API calls for development.
type StripeStub struct{}

// CreatePaymentIntent simulates creating a Stripe PaymentIntent.
func (s *StripeStub) CreatePaymentIntent(amount float64, currency string) (string, error) {
	// Simulate occasional failures for resilience testing
	if rand.Float64() < 0.02 { // 2% failure rate
		return "", fmt.Errorf("stripe: card declined (simulated)")
	}

	// Generate a mock transaction ID
	txnID := fmt.Sprintf("pi_%s_%d", generateRandomString(16), time.Now().UnixMilli())
	log.Info().
		Float64("amount", amount).
		Str("currency", currency).
		Str("txn_id", txnID).
		Msg("stripe: payment intent created (stub)")

	return txnID, nil
}

// CapturePayment simulates capturing a payment.
func (s *StripeStub) CapturePayment(txnID string) error {
	log.Info().Str("txn_id", txnID).Msg("stripe: payment captured (stub)")
	return nil
}

// Refund simulates a refund.
func (s *StripeStub) Refund(txnID string, amount float64) (string, error) {
	refundID := fmt.Sprintf("re_%s", generateRandomString(16))
	log.Info().
		Str("txn_id", txnID).
		Float64("amount", amount).
		Str("refund_id", refundID).
		Msg("stripe: refund processed (stub)")
	return refundID, nil
}

func generateRandomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

// Repository handles payment database operations.
type Repository struct {
	pool *pgxpool.Pool
	q    db.DBTX
}

// NewRepository creates a new payment repository.
func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool, q: pool}
}

// Create inserts a new payment record.
func (r *Repository) Create(ctx context.Context, payment *models.Payment) error {
	err := r.q.QueryRow(ctx,
		`INSERT INTO payments (ride_id, amount, currency, status, stripe_txn_id)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id, created_at`,
		payment.RideID, payment.Amount, payment.Currency, payment.Status, payment.StripeTxnID,
	).Scan(&payment.ID, &payment.CreatedAt)

	if err != nil {
		return fmt.Errorf("create payment: %w", err)
	}
	return nil
}

// GetByRideID retrieves the payment for a ride.
func (r *Repository) GetByRideID(ctx context.Context, rideID uuid.UUID) (*models.Payment, error) {
	p := &models.Payment{}
	err := r.q.QueryRow(ctx,
		`SELECT id, ride_id, amount, currency, status, stripe_txn_id, created_at
		 FROM payments WHERE ride_id = $1`, rideID,
	).Scan(&p.ID, &p.RideID, &p.Amount, &p.Currency, &p.Status, &p.StripeTxnID, &p.CreatedAt)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get payment: %w", err)
	}
	return p, nil
}

// UpdateStatus updates a payment's status.
func (r *Repository) UpdateStatus(ctx context.Context, id uuid.UUID, status string) error {
	_, err := r.q.Exec(ctx,
		`UPDATE payments SET status = $2 WHERE id = $1`,
		id, status,
	)
	return err
}

// Service implements payment business logic.
type Service struct {
	repo   RepositoryInterface
	stripe PaymentGateway
}

// NewService creates a new payment service.
func NewService(repo RepositoryInterface) *Service {
	return &Service{
		repo:   repo,
		stripe: &StripeStub{},
	}
}

// ChargeRide processes the payment for a completed ride.
func (s *Service) ChargeRide(ctx context.Context, rideID uuid.UUID, amount float64) (*models.Payment, error) {
	ctx, span := tracing.Tracer("payment").Start(ctx, "ChargeRide")
	defer span.End()

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Check if already charged (idempotent)
	existing, _ := s.repo.GetByRideID(ctx, rideID)
	if existing != nil && existing.Status == "completed" {
		return existing, nil
	}

	// Create Stripe payment intent
	txnID, err := s.stripe.CreatePaymentIntent(amount, "USD")
	if err != nil {
		return nil, apperrors.Internal("payment processing failed", err)
	}

	// Capture the payment
	if err := s.stripe.CapturePayment(txnID); err != nil {
		return nil, apperrors.Internal("payment capture failed", err)
	}

	// Record payment
	payment := &models.Payment{
		RideID:      rideID,
		Amount:      amount,
		Currency:    "USD",
		Status:      "completed",
		StripeTxnID: txnID,
	}

	if err := s.repo.Create(ctx, payment); err != nil {
		return nil, apperrors.Internal("failed to record payment", err)
	}

	log.Info().
		Str("ride_id", rideID.String()).
		Float64("amount", amount).
		Str("txn_id", txnID).
		Msg("ride payment completed")

	return payment, nil
}

// GetPayment retrieves payment details for a ride.
func (s *Service) GetPayment(ctx context.Context, rideID uuid.UUID) (*models.Payment, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	p, err := s.repo.GetByRideID(ctx, rideID)
	if err != nil {
		return nil, apperrors.Internal("failed to get payment", err)
	}
	if p == nil {
		return nil, apperrors.NotFound("payment not found")
	}
	return p, nil
}
