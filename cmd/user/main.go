package main

import (
	"context"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/hugovillarreal/ridesharing/internal/user"
	"github.com/hugovillarreal/ridesharing/pkg/config"
	"github.com/hugovillarreal/ridesharing/pkg/db"
	"github.com/hugovillarreal/ridesharing/pkg/logger"
	"github.com/hugovillarreal/ridesharing/pkg/pb"

	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
)

func main() {
	// Initialize logger
	logger.Init("info")

	// Load configuration
	cfg := config.Load()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// ── Connect to PostgreSQL ──────────────────────────────────
	pool, err := db.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect to database")
	}
	defer pool.Close()

	// ── Initialize repositories & services ─────────────────────
	userRepo := user.NewRepository(pool)
	userSvc := user.NewService(userRepo)

	// ── Initialize gRPC Server ──────────────────────────────────
	grpcServer := grpc.NewServer()
	userGrpcServer := user.NewGRPCServer(userSvc)
	pb.RegisterUserServiceServer(grpcServer, userGrpcServer)

	// ── Start server ────────────────────────────────────────────
	addr := ":" + cfg.ServerPort
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to listen on port")
	}

	go func() {
		log.Info().Str("port", cfg.ServerPort).Msg("Starting User gRPC microservice")
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatal().Err(err).Msg("failed to serve gRPC")
		}
	}()

	// ── Graceful shutdown ───────────────────────────────────────
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Info().Msg("shutting down User gRPC server...")

	grpcServer.GracefulStop()
	log.Info().Msg("User gRPC server exited gracefully")
}
