package location

import (
	"context"

	"github.com/google/uuid"
	"github.com/hugovillarreal/ridesharing/pkg/pb"
)

// GRPCServer implements the pb.LocationServiceServer interface.
type GRPCServer struct {
	pb.UnimplementedLocationServiceServer
	svc *Service
}

// NewGRPCServer returns a new initialized GRPCServer.
func NewGRPCServer(svc *Service) *GRPCServer {
	return &GRPCServer{svc: svc}
}

// UpdateLocation calls the underlying business logic to update a driver's GPS coordinates.
func (s *GRPCServer) UpdateLocation(ctx context.Context, req *pb.UpdateLocationRequest) (*pb.UpdateLocationResponse, error) {
	userId, err := uuid.Parse(req.UserId)
	if err != nil {
		return nil, err
	}

	err = s.svc.UpdateLocation(ctx, userId, req.Lat, req.Long)
	if err != nil {
		return nil, err
	}
	return &pb.UpdateLocationResponse{Success: true}, nil
}

// GetNearbyDrivers calls the underlying business logic to find drivers within a radius.
func (s *GRPCServer) GetNearbyDrivers(ctx context.Context, req *pb.GetNearbyDriversRequest) (*pb.GetNearbyDriversResponse, error) {
	drivers, err := s.svc.GetNearbyDrivers(ctx, req.Lat, req.Long, req.RadiusKm)
	if err != nil {
		return nil, err
	}

	var pbDrivers []*pb.NearbyDriver
	for _, d := range drivers {
		pbDrivers = append(pbDrivers, &pb.NearbyDriver{
			DriverId:     d.DriverID.String(),
			Lat:          d.Lat,
			Long:         d.Long,
			DistanceKm:   d.DistanceKm,
			EstimatedEta: int32(d.EstimatedETA),
		})
	}

	return &pb.GetNearbyDriversResponse{Drivers: pbDrivers}, nil
}

// ActiveDriverCount calls the underlying business logic to count available drivers for surge pricing.
func (s *GRPCServer) ActiveDriverCount(ctx context.Context, req *pb.ActiveDriverCountRequest) (*pb.ActiveDriverCountResponse, error) {
	count, err := s.svc.ActiveDriverCount(ctx, req.Lat, req.Long, req.RadiusKm)
	if err != nil {
		return nil, err
	}

	return &pb.ActiveDriverCountResponse{Count: int32(count)}, nil
}
