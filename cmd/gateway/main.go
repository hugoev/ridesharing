package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/hugovillarreal/ridesharing/internal/auth"
	"github.com/hugovillarreal/ridesharing/internal/location"
	"github.com/hugovillarreal/ridesharing/internal/notification"
	"github.com/hugovillarreal/ridesharing/internal/payment"
	"github.com/hugovillarreal/ridesharing/internal/ride"
	"github.com/hugovillarreal/ridesharing/internal/user"
	"github.com/hugovillarreal/ridesharing/pkg/config"
	"github.com/hugovillarreal/ridesharing/pkg/db"
	"github.com/hugovillarreal/ridesharing/pkg/kafka"
	"github.com/hugovillarreal/ridesharing/pkg/logger"
	"github.com/hugovillarreal/ridesharing/pkg/metrics"
	"github.com/hugovillarreal/ridesharing/pkg/middleware"
	"github.com/hugovillarreal/ridesharing/pkg/pb"
	redisclient "github.com/hugovillarreal/ridesharing/pkg/redis"
	"github.com/hugovillarreal/ridesharing/pkg/security"
	"github.com/hugovillarreal/ridesharing/pkg/tracing"
)

func main() {
	// Initialize logger
	logger.Init("info")

	// Load configuration
	cfg := config.Load()

	ctx, cancel := context.WithCancel(context.Background())

	// ── Initialize OpenTelemetry tracing ────────────────────────
	tracerShutdown, err := tracing.Init(ctx, tracing.Config{
		ServiceName:    "rideshare-gateway",
		ServiceVersion: "1.0.0",
		Endpoint:       getEnvOrDefault("OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:4318"),
		Enabled:        os.Getenv("OTEL_ENABLED") != "false",
	})
	if err != nil {
		log.Warn().Err(err).Msg("failed to initialize tracing, continuing without it")
	}
	defer cancel()

	// ── Connect to PostgreSQL (For Healthcheck only) ───────────
	pool, err := db.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect to database")
	}
	defer pool.Close()

	// ── Connect to Redis (For Healthcheck only) ─────────────────
	rdb, err := redisclient.NewClient(ctx, cfg.RedisAddr, cfg.RedisPassword, cfg.RedisDB)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect to redis")
	}
	defer rdb.Close()

	// ── Initialize gRPC Clients ─────────────────────────────────
	authConn, err := grpc.NewClient(cfg.AuthServiceAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect to auth service")
	}
	defer authConn.Close()
	authClient := pb.NewAuthServiceClient(authConn)

	userConn, err := grpc.NewClient(cfg.UserServiceAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect to user service")
	}
	defer userConn.Close()
	userClient := pb.NewUserServiceClient(userConn)

	rideConn, err := grpc.NewClient(cfg.RideServiceAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect to ride service")
	}
	defer rideConn.Close()
	rideClient := pb.NewRideServiceClient(rideConn)

	locationConn, err := grpc.NewClient(cfg.LocationServiceAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect to location service")
	}
	defer locationConn.Close()
	locationClient := pb.NewLocationServiceClient(locationConn)

	paymentConn, err := grpc.NewClient(cfg.PaymentServiceAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect to payment service")
	}
	defer paymentConn.Close()
	paymentClient := pb.NewPaymentServiceClient(paymentConn)

	// ── Initialize notification hub ─────────────────────────────
	notifHub := notification.NewHub()
	go notifHub.Run()

	// ── Initialize Proxy Handlers ───────────────────────────────
	authHandler := auth.NewHandler(authClient)
	userHandler := user.NewHandler(userClient)
	rideHandler := ride.NewHandler(rideClient)
	locationHandler := location.NewHandler(locationClient)
	paymentHandler := payment.NewHandler(paymentClient)
	notifHandler := notification.NewHandler(notifHub, cfg.JWTSecret)

	// ── Setup Kafka consumer for ride events ────────────────────
	rideConsumer := kafka.NewConsumer(cfg.KafkaBrokers, "ride-events", "notification-service")
	go func() {
		rideConsumer.Consume(ctx, notifHandler.KafkaEventHandler)
	}()
	defer rideConsumer.Close()

	// ── Setup Gin router ────────────────────────────────────────
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()

	// Global middleware
	router.Use(
		middleware.Recovery(),
		middleware.RequestID(),
		otelgin.Middleware("rideshare-gateway"), // OpenTelemetry tracing
		middleware.TraceID(),                    // Inject trace ID into response header
		middleware.CORS(),
		middleware.MaxBodySize(1<<20), // 1MB max request body
		middleware.RateLimit(cfg.RateLimit, cfg.RateBurst),
		metrics.Middleware(), // Prometheus HTTP metrics
		ginLogger(),
	)

	// ── Observability endpoints ──────────────────────────────────
	router.GET("/health", healthCheck(pool, rdb))
	router.GET("/metrics", metrics.Handler())

	// API v1 routes
	v1 := router.Group("/v1")
	{
		// Public routes (no auth required)
		authHandler.RegisterRoutes(v1)

		// Protected routes (auth + per-user rate limit)
		userRL := security.NewUserRateLimiter(100, 1*time.Minute) // 100 req/min per user
		protected := v1.Group("")
		protected.Use(middleware.Auth(cfg.JWTSecret))
		protected.Use(userRL.Middleware())
		{
			userHandler.RegisterRoutes(protected)
			rideHandler.RegisterRoutes(protected)
			locationHandler.RegisterRoutes(protected)
			paymentHandler.RegisterRoutes(protected)
		}
	}

	// Internal/system routes
	router.GET("/ws", notifHandler.WebSocketHandler)

	// ── Start server ────────────────────────────────────────────
	addr := ":" + cfg.ServerPort
	srv := &http.Server{
		Addr:    addr,
		Handler: router,
	}

	go func() {
		log.Info().Str("port", cfg.ServerPort).Msg("Starting API Gateway (BFF) server")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("failed to start server")
		}
	}()

	// ── Graceful shutdown ───────────────────────────────────────
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Info().Msg("shutting down server...")

	ctxShutdown, cancelShutdown := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelShutdown()

	if err := srv.Shutdown(ctxShutdown); err != nil {
		log.Error().Err(err).Msg("server forced to shutdown")
	}

	tracerShutdown(ctxShutdown)

	log.Info().Msg("server exited gracefully")
}

// healthCheck returns a basic health summary including Postgres and Redis connectivity.
func healthCheck(dbPool *pgxpool.Pool, rdb *redis.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()

		dbStatus := "down"
		if err := dbPool.Ping(ctx); err == nil {
			dbStatus = "up"
		}

		redisStatus := "down"
		if err := rdb.Ping(ctx).Err(); err == nil {
			redisStatus = "up"
		}

		status := http.StatusOK
		if dbStatus == "down" || redisStatus == "down" {
			status = http.StatusServiceUnavailable
		}

		c.JSON(status, gin.H{
			"status":   "ok",
			"database": dbStatus,
			"redis":    redisStatus,
			"version":  "1.0.0",
		})
	}
}

func getEnvOrDefault(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}

// ginLogger bridge to zerolog
func ginLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		duration := time.Since(start)

		log.Info().
			Str("method", c.Request.Method).
			Str("path", c.Request.URL.Path).
			Int("status", c.Writer.Status()).
			Dur("latency", duration).
			Str("ip", c.ClientIP()).
			Msg("HTTP Request")
	}
}
