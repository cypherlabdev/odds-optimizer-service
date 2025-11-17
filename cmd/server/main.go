package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/cypherlabdev/odds-optimizer-service/internal/cache"
	"github.com/cypherlabdev/odds-optimizer-service/internal/config"
	httpHandler "github.com/cypherlabdev/odds-optimizer-service/internal/handler/http"
	"github.com/cypherlabdev/odds-optimizer-service/internal/messaging"
	"github.com/cypherlabdev/odds-optimizer-service/internal/service"
	"github.com/cypherlabdev/odds-optimizer-service/pkg/optimizer"
)

func main() {
	// Load configuration
	cfg, err := config.LoadConfig("config/config.yaml")
	if err != nil {
		log.Fatal().Err(err).Msg("failed to load config")
	}

	// Setup logger
	logger := setupLogger(cfg.Logging)
	logger.Info().Msg("starting odds-optimizer-service")

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create Redis cache
	redisCache := cache.NewRedisCache(
		cache.RedisCacheConfig{
			Addr:     cfg.Redis.Addr,
			Password: cfg.Redis.Password,
			DB:       cfg.Redis.DB,
			TTL:      cfg.Redis.TTL,
		},
		logger,
	)
	defer redisCache.Close()

	// Test Redis connection
	if err := redisCache.Ping(ctx); err != nil {
		logger.Fatal().Err(err).Msg("failed to connect to Redis")
	}
	logger.Info().Str("addr", cfg.Redis.Addr).Msg("connected to Redis")

	// Create optimizer
	opt := optimizer.NewOptimizer(
		cfg.Optimization.ToOptimizationParams(),
		logger,
	)
	logger.Info().Msg("optimizer initialized")

	// Create optimizer service layer
	optimizerService := service.NewOptimizerService(opt, redisCache, logger)
	logger.Info().Msg("optimizer service initialized")

	// Create Kafka consumer
	consumer := messaging.NewKafkaConsumer(
		messaging.KafkaConsumerConfig{
			Brokers: cfg.Kafka.Brokers,
			Topic:   cfg.Kafka.Topic,
			GroupID: cfg.Kafka.GroupID,
		},
		opt,
		redisCache,
		logger,
	)
	defer consumer.Close()

	// Start Kafka consumer in goroutine
	go func() {
		if err := consumer.Start(ctx); err != nil {
			logger.Error().Err(err).Msg("Kafka consumer failed")
		}
	}()

	// Initialize HTTP handler
	oddsHandler := httpHandler.NewOddsHandler(optimizerService, logger)
	logger.Info().Msg("HTTP handler initialized")

	// Setup HTTP server routes
	mux := http.NewServeMux()

	// Health and monitoring endpoints
	mux.HandleFunc("/health", healthHandler)
	mux.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		readyHandler(w, r, redisCache)
	})
	mux.Handle("/metrics", promhttp.Handler())

	// Register API routes
	oddsHandler.RegisterRoutes(mux)
	logger.Info().Msg("API routes registered")

	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:      mux,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	// Start HTTP server in goroutine
	go func() {
		logger.Info().Int("port", cfg.Server.Port).Msg("starting HTTP server")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error().Err(err).Msg("HTTP server failed")
		}
	}()

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	logger.Info().Msg("shutting down gracefully...")

	// Cancel context to stop consumer
	cancel()

	// Shutdown HTTP server
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error().Err(err).Msg("HTTP server shutdown failed")
	}

	logger.Info().Msg("shutdown complete")
}

// setupLogger configures the logger based on config
func setupLogger(cfg config.LoggingConfig) zerolog.Logger {
	// Set log level
	level, err := zerolog.ParseLevel(cfg.Level)
	if err != nil {
		level = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(level)

	// Set format
	if cfg.Format == "console" {
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339})
	}

	return log.Logger.With().Str("service", "odds-optimizer").Logger()
}

// healthHandler returns 200 if service is running
func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

// readyHandler returns 200 if service is ready to accept traffic
func readyHandler(w http.ResponseWriter, r *http.Request, cache *cache.RedisCache) {
	// Check Redis connection
	if err := cache.Ping(r.Context()); err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte("Redis unavailable"))
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("READY"))
}
