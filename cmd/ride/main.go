package main

import (
	"context"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/hugovillarreal/ridesharing/internal/location"
	"github.com/hugovillarreal/ridesharing/internal/ride"
	"github.com/hugovillarreal/ridesharing/pkg/config"
	"github.com/hugovillarreal/ridesharing/pkg/db"
	"github.com/hugovillarreal/ridesharing/pkg/kafka"
	"github.com/hugovillarreal/ridesharing/pkg/logger"
	"github.com/hugovillarreal/ridesharing/pkg/pb"
	"github.com/hugovillarreal/ridesharing/pkg/redis"

	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
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

	// ── Connect to Redis ────────────────────────────────────────
	rdb, err := redis.NewClient(ctx, cfg.RedisAddr, cfg.RedisPassword, cfg.RedisDB)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect to redis")
	}
	defer rdb.Close()

	// ── Setup Kafka producer ────────────────────────────────────
	rideProducer := kafka.NewProducer(cfg.KafkaBrokers, "ride-events")
	defer rideProducer.Close()

	// ── Setup Location gRPC Client Adapter ──────────────────────
	locationConn, err := grpc.NewClient(cfg.LocationServiceAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect to location service")
	}
	defer locationConn.Close()
	locationClientAdapter := location.NewGRPCClient(locationConn)

	// ── Initialize repositories & services ─────────────────────
	rideRepo := ride.NewRepository(pool)
	rideSvc := ride.NewService(rideRepo, pool, locationClientAdapter, rideProducer, rdb)

	// ── Initialize gRPC Server ──────────────────────────────────
	grpcServer := grpc.NewServer()
	rideGrpcServer := ride.NewGRPCServer(rideSvc)
	pb.RegisterRideServiceServer(grpcServer, rideGrpcServer)

	// ── Start server ────────────────────────────────────────────
	addr := ":" + cfg.ServerPort
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to listen on port")
	}

	go func() {
		log.Info().Str("port", cfg.ServerPort).Msg("Starting Ride gRPC microservice")
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatal().Err(err).Msg("failed to serve gRPC")
		}
	}()

	// ── Graceful shutdown ───────────────────────────────────────
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Info().Msg("shutting down Ride gRPC server...")

	grpcServer.GracefulStop()
	log.Info().Msg("Ride gRPC server exited gracefully")
}
