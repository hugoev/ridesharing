package auth

import (
	"context"
	"errors"

	apperrors "github.com/hugovillarreal/ridesharing/pkg/errors"
	"github.com/hugovillarreal/ridesharing/pkg/pb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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
		return nil, mapError(err)
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
		return nil, mapError(err)
	}
	return buildAuthResponse(resp), nil
}

// Refresh calls the underlying business logic to refresh a JWT.
func (s *GRPCServer) Refresh(ctx context.Context, req *pb.RefreshRequest) (*pb.AuthResponse, error) {
	resp, err := s.svc.RefreshToken(ctx, req.RefreshToken)
	if err != nil {
		return nil, mapError(err)
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

func mapError(err error) error {
	if err == nil {
		return nil
	}
	var appErr *apperrors.AppError
	if errors.As(err, &appErr) {
		switch appErr.Code {
		case 400:
			return status.Error(codes.InvalidArgument, appErr.Message)
		case 401:
			return status.Error(codes.Unauthenticated, appErr.Message)
		case 403:
			return status.Error(codes.PermissionDenied, appErr.Message)
		case 404:
			return status.Error(codes.NotFound, appErr.Message)
		case 409:
			return status.Error(codes.AlreadyExists, appErr.Message)
		case 429:
			return status.Error(codes.ResourceExhausted, appErr.Message)
		default:
			return status.Error(codes.Internal, appErr.Message)
		}
	}
	return status.Error(codes.Internal, err.Error())
}
