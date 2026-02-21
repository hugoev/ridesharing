package user

import (
	"context"

	"github.com/google/uuid"
	"github.com/hugovillarreal/ridesharing/internal/models"
	"github.com/hugovillarreal/ridesharing/pkg/pb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// GRPCServer implements the pb.UserServiceServer interface.
type GRPCServer struct {
	pb.UnimplementedUserServiceServer
	svc *Service
}

// NewGRPCServer returns a new initialized GRPCServer.
func NewGRPCServer(svc *Service) *GRPCServer {
	return &GRPCServer{svc: svc}
}

// GetProfile calls the underlying business logic to retrieve a user profile.
func (s *GRPCServer) GetProfile(ctx context.Context, req *pb.GetProfileRequest) (*pb.GetProfileResponse, error) {
	userId, err := uuid.Parse(req.UserId)
	if err != nil {
		return nil, err
	}

	usr, err := s.svc.GetProfile(ctx, userId)
	if err != nil {
		return nil, err
	}
	return buildGetProfileResponse(usr), nil
}

// UpdateProfile calls the underlying business logic to update a user profile.
func (s *GRPCServer) UpdateProfile(ctx context.Context, req *pb.UpdateProfileRequest) (*pb.GetProfileResponse, error) {
	userId, err := uuid.Parse(req.UserId)
	if err != nil {
		return nil, err
	}

	// Build the update request from optional fields
	updateReq := UpdateProfileRequest{}
	if req.Name != nil {
		updateReq.Name = *req.Name
	}
	if req.Phone != nil {
		updateReq.Phone = *req.Phone
	}
	if req.VehicleType != nil {
		updateReq.VehicleType = *req.VehicleType
	}
	if req.LicensePlate != nil {
		updateReq.LicensePlate = *req.LicensePlate
	}

	usr, err := s.svc.UpdateProfile(ctx, userId, updateReq)
	if err != nil {
		return nil, err
	}
	return buildGetProfileResponse(usr), nil
}

// SetAvailability calls the underlying business logic to toggle a driver's availability.
func (s *GRPCServer) SetAvailability(ctx context.Context, req *pb.SetAvailabilityRequest) (*pb.SetAvailabilityResponse, error) {
	userId, err := uuid.Parse(req.UserId)
	if err != nil {
		return nil, err
	}

	err = s.svc.SetAvailability(ctx, userId, req.IsAvailable)
	if err != nil {
		return nil, err
	}
	return &pb.SetAvailabilityResponse{Success: true}, nil
}

func buildGetProfileResponse(usr *models.UserWithDriver) *pb.GetProfileResponse {
	resp := &pb.GetProfileResponse{
		User: &pb.User{
			Id:        usr.ID.String(),
			Email:     usr.Email,
			Name:      usr.Name,
			Phone:     usr.Phone,
			Role:      usr.Role,
			CreatedAt: timestamppb.New(usr.CreatedAt),
			UpdatedAt: timestamppb.New(usr.UpdatedAt),
		},
	}

	if usr.Driver != nil {
		resp.Driver = &pb.Driver{
			UserId:       usr.Driver.UserID.String(),
			VehicleType:  usr.Driver.VehicleType,
			LicensePlate: usr.Driver.LicensePlate,
			IsAvailable:  usr.Driver.IsAvailable,
			Rating:       usr.Driver.Rating,
			CreatedAt:    timestamppb.New(usr.Driver.CreatedAt),
			UpdatedAt:    timestamppb.New(usr.Driver.UpdatedAt),
		}
	}
	return resp
}
