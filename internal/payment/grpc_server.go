package payment

import (
	"context"

	"github.com/google/uuid"
	"github.com/hugovillarreal/ridesharing/internal/models"
	"github.com/hugovillarreal/ridesharing/pkg/pb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// GRPCServer implements the pb.PaymentServiceServer interface.
type GRPCServer struct {
	pb.UnimplementedPaymentServiceServer
	svc *Service
}

// NewGRPCServer returns a new initialized GRPCServer.
func NewGRPCServer(svc *Service) *GRPCServer {
	return &GRPCServer{svc: svc}
}

// ChargeRide calls the underlying business logic to handle a stripe charge.
func (s *GRPCServer) ChargeRide(ctx context.Context, req *pb.ChargeRideRequest) (*pb.ChargeRideResponse, error) {
	rideId, err := uuid.Parse(req.RideId)
	if err != nil {
		return nil, err
	}

	payment, err := s.svc.ChargeRide(ctx, rideId, req.Amount)
	if err != nil {
		return nil, err
	}

	return &pb.ChargeRideResponse{
		Payment: buildPBPayment(payment),
	}, nil
}

// GetPayment calls the underlying business logic to fetch a payment record.
func (s *GRPCServer) GetPayment(ctx context.Context, req *pb.GetPaymentRequest) (*pb.GetPaymentResponse, error) {
	rideId, err := uuid.Parse(req.RideId)
	if err != nil {
		return nil, err
	}

	payment, err := s.svc.GetPayment(ctx, rideId)
	if err != nil {
		return nil, err
	}

	return &pb.GetPaymentResponse{
		Payment: buildPBPayment(payment),
	}, nil
}

func buildPBPayment(payment *models.Payment) *pb.Payment {
	return &pb.Payment{
		Id:          payment.ID.String(),
		RideId:      payment.RideID.String(),
		Amount:      payment.Amount,
		Currency:    payment.Currency,
		Status:      payment.Status,
		StripeTxnId: payment.StripeTxnID,
		CreatedAt:   timestamppb.New(payment.CreatedAt),
	}
}
