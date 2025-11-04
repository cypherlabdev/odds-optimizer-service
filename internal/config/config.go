package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/shopspring/decimal"
	"github.com/spf13/viper"

	"github.com/cypherlabdev/odds-optimizer-service/internal/models"
)

// Config holds all configuration for odds-optimizer-service
type Config struct {
	Server       ServerConfig
	Kafka        KafkaConfig
	Redis        RedisConfig
	Optimization OptimizationConfig
	Logging      LoggingConfig
}

// ServerConfig holds HTTP server configuration
type ServerConfig struct {
	Port         int
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

// KafkaConfig holds Kafka configuration
type KafkaConfig struct {
	Brokers []string
	Topic   string // Topic to consume from (normalized_odds)
	GroupID string
}

// RedisConfig holds Redis configuration
type RedisConfig struct {
	Addr     string
	Password string
	DB       int
	TTL      time.Duration
}

// OptimizationConfig holds optimization parameters
type OptimizationConfig struct {
	MinMargin        float64 // Minimum profit margin (0.02 = 2%)
	MaxMargin        float64 // Maximum profit margin (0.10 = 10%)
	MinSpread        float64 // Minimum back-lay spread
	TargetConfidence float64 // Target confidence level (0-1)
}

// LoggingConfig holds logging configuration
type LoggingConfig struct {
	Level  string // debug, info, warn, error
	Format string // json, console
}

// LoadConfig loads configuration from file and environment variables
func LoadConfig(configPath string) (*Config, error) {
	v := viper.New()

	// Set defaults
	v.SetDefault("server.port", 8081)
	v.SetDefault("server.read_timeout", 30*time.Second)
	v.SetDefault("server.write_timeout", 30*time.Second)

	v.SetDefault("kafka.brokers", []string{"localhost:9092"})
	v.SetDefault("kafka.topic", "normalized_odds")
	v.SetDefault("kafka.group_id", "odds-optimizer")

	v.SetDefault("redis.addr", "localhost:6379")
	v.SetDefault("redis.password", "")
	v.SetDefault("redis.db", 0)
	v.SetDefault("redis.ttl", 15*time.Minute)

	v.SetDefault("optimization.min_margin", 0.02)
	v.SetDefault("optimization.max_margin", 0.10)
	v.SetDefault("optimization.min_spread", 0.05)
	v.SetDefault("optimization.target_confidence", 0.85)

	v.SetDefault("logging.level", "info")
	v.SetDefault("logging.format", "json")

	// Read config file if provided
	if configPath != "" {
		v.SetConfigFile(configPath)
		if err := v.ReadInConfig(); err != nil {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
	}

	// Override with environment variables
	v.SetEnvPrefix("ODDS_OPTIMIZER")
	v.AutomaticEnv()
	// Replace . with _ for environment variables
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// Unmarshal to struct
	var config Config
	if err := v.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &config, nil
}

// ToOptimizationParams converts config to optimization parameters
func (c *OptimizationConfig) ToOptimizationParams() models.OptimizationParams {
	return models.OptimizationParams{
		MinMargin:        decimal.NewFromFloat(c.MinMargin),
		MaxMargin:        decimal.NewFromFloat(c.MaxMargin),
		MinSpread:        decimal.NewFromFloat(c.MinSpread),
		TargetConfidence: c.TargetConfidence,
	}
}
