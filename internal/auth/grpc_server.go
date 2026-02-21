package auth

import (
	"context"

	"github.com/hugovillarreal/ridesharing/pkg/pb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// GRPCServer implements the pb.AuthServiceServer interface.
type GRPCServer struct {
	pb.UnimplementedAuthServiceServer
	svc *Service
}

// NewGRPCServer returns a new initialized GRPCServer.
func NewGRPCServer(svc *Service) *GRPCServer {
	return &GRPCServer{svc: svc}
}

// Register calls the underlying business logic to register a user.
func (s *GRPCServer) Register(ctx context.Context, req *pb.RegisterRequest) (*pb.AuthResponse, error) {
	resp, err := s.svc.Register(ctx, RegisterRequest{
		Email:    req.Email,
		Password: req.Password,
		Name:     req.Name,
		Phone:    req.Phone,
		Role:     req.Role,
	})
	if err != nil {
		return nil, err
	}
	return buildAuthResponse(resp), nil
}

// Login calls the underlying business logic to authenticate a user.
func (s *GRPCServer) Login(ctx context.Context, req *pb.LoginRequest) (*pb.AuthResponse, error) {
	resp, err := s.svc.Login(ctx, LoginRequest{
		Email:    req.Email,
		Password: req.Password,
	})
	if err != nil {
		return nil, err
	}
	return buildAuthResponse(resp), nil
}

// Refresh calls the underlying business logic to refresh a JWT.
func (s *GRPCServer) Refresh(ctx context.Context, req *pb.RefreshRequest) (*pb.AuthResponse, error) {
	resp, err := s.svc.RefreshToken(ctx, req.RefreshToken)
	if err != nil {
		return nil, err
	}
	return buildAuthResponse(resp), nil
}

func buildAuthResponse(resp *AuthResponse) *pb.AuthResponse {
	return &pb.AuthResponse{
		AccessToken:  resp.AccessToken,
		RefreshToken: resp.RefreshToken,
		ExpiresIn:    int32(resp.ExpiresIn),
		User: &pb.User{
			Id:        resp.User.ID.String(),
			Email:     resp.User.Email,
			Name:      resp.User.Name,
			Phone:     resp.User.Phone,
			Role:      resp.User.Role,
			CreatedAt: timestamppb.New(resp.User.CreatedAt),
			UpdatedAt: timestamppb.New(resp.User.UpdatedAt),
		},
	}
}
