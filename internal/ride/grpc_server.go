package ride

import (
	"context"

	"github.com/google/uuid"
	"github.com/hugovillarreal/ridesharing/internal/models"
	"github.com/hugovillarreal/ridesharing/pkg/pb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// GRPCServer implements the pb.RideServiceServer interface.
type GRPCServer struct {
	pb.UnimplementedRideServiceServer
	svc *Service
}

// NewGRPCServer returns a new initialized GRPCServer.
func NewGRPCServer(svc *Service) *GRPCServer {
	return &GRPCServer{svc: svc}
}

// RequestRide calls the underlying business logic to request a new ride.
func (s *GRPCServer) RequestRide(ctx context.Context, req *pb.RideRequest) (*pb.RideResponse, error) {
	riderId, err := uuid.Parse(req.RiderId)
	if err != nil {
		return nil, err
	}

	rideReq := RideRequest{
		PickupLat:   req.PickupLat,
		PickupLong:  req.PickupLong,
		DropoffLat:  req.DropoffLat,
		DropoffLong: req.DropoffLong,
	}

	resp, err := s.svc.RequestRide(ctx, riderId, rideReq)
	if err != nil {
		return nil, err
	}
	return &pb.RideResponse{Ride: buildPBRide(resp.Ride)}, nil
}

// AcceptRide calls the underlying business logic for a driver to accept a ride.
func (s *GRPCServer) AcceptRide(ctx context.Context, req *pb.AcceptRideRequest) (*pb.RideResponse, error) {
	rideId, err := uuid.Parse(req.RideId)
	if err != nil {
		return nil, err
	}
	driverId, err := uuid.Parse(req.DriverId)
	if err != nil {
		return nil, err
	}

	ride, err := s.svc.AcceptRide(ctx, rideId, driverId)
	if err != nil {
		return nil, err
	}
	return &pb.RideResponse{Ride: buildPBRide(ride)}, nil
}

// CompleteRide calls the underlying business logic to mark a ride as completed.
func (s *GRPCServer) CompleteRide(ctx context.Context, req *pb.CompleteRideRequest) (*pb.RideResponse, error) {
	rideId, err := uuid.Parse(req.RideId)
	if err != nil {
		return nil, err
	}
	driverId, err := uuid.Parse(req.DriverId)
	if err != nil {
		return nil, err
	}

	ride, err := s.svc.CompleteRide(ctx, rideId, driverId)
	if err != nil {
		return nil, err
	}
	return &pb.RideResponse{Ride: buildPBRide(ride)}, nil
}

// CancelRide calls the underlying business logic to cancel a ride.
func (s *GRPCServer) CancelRide(ctx context.Context, req *pb.CancelRideRequest) (*pb.RideResponse, error) {
	rideId, err := uuid.Parse(req.RideId)
	if err != nil {
		return nil, err
	}
	userId, err := uuid.Parse(req.UserId)
	if err != nil {
		return nil, err
	}

	ride, err := s.svc.CancelRide(ctx, rideId, userId)
	if err != nil {
		return nil, err
	}
	return &pb.RideResponse{Ride: buildPBRide(ride)}, nil
}

// GetRide retrieves a single ride's details.
func (s *GRPCServer) GetRide(ctx context.Context, req *pb.GetRideRequest) (*pb.RideResponse, error) {
	rideId, err := uuid.Parse(req.RideId)
	if err != nil {
		return nil, err
	}

	ride, err := s.svc.GetRide(ctx, rideId)
	if err != nil {
		return nil, err
	}
	return &pb.RideResponse{Ride: buildPBRide(ride)}, nil
}

// RideHistory retrieves a user's rides.
func (s *GRPCServer) RideHistory(ctx context.Context, req *pb.RideHistoryRequest) (*pb.RideHistoryResponse, error) {
	userId, err := uuid.Parse(req.UserId)
	if err != nil {
		return nil, err
	}

	rides, total, err := s.svc.GetRideHistory(ctx, userId, int(req.Limit), int(req.Offset))
	if err != nil {
		return nil, err
	}

	var pbRides []*pb.Ride
	for _, r := range rides {
		r := r
		pbRides = append(pbRides, buildPBRide(&r))
	}

	return &pb.RideHistoryResponse{
		Rides: pbRides,
		Total: int32(total),
	}, nil
}

func buildPBRide(r *models.Ride) *pb.Ride {
	pbRide := &pb.Ride{
		Id:              r.ID.String(),
		RiderId:         r.RiderID.String(),
		PickupLat:       r.PickupLat,
		PickupLong:      r.PickupLong,
		DropoffLat:      r.DropoffLat,
		DropoffLong:     r.DropoffLong,
		Status:          string(r.Status),
		Fare:            r.Fare,
		SurgeMultiplier: r.SurgeMultiplier,
		CreatedAt:       timestamppb.New(r.CreatedAt),
		UpdatedAt:       timestamppb.New(r.UpdatedAt),
	}

	if r.DriverID != nil {
		driverStr := r.DriverID.String()
		pbRide.DriverId = &driverStr
	}
	if r.EstimatedETA != nil {
		eta := int32(*r.EstimatedETA)
		pbRide.EstimatedEta = &eta
	}
	if r.DistanceKm != nil {
		dist := *r.DistanceKm
		pbRide.DistanceKm = &dist
	}
	if r.StartedAt != nil {
		pbRide.StartedAt = timestamppb.New(*r.StartedAt)
	}
	if r.EndedAt != nil {
		pbRide.EndedAt = timestamppb.New(*r.EndedAt)
	}

	return pbRide
}
