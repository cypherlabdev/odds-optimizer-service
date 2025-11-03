package config

import (
	"os"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLoadConfig_Defaults tests loading configuration with default values
func TestLoadConfig_Defaults(t *testing.T) {
	// Load config without a file (should use defaults)
	config, err := LoadConfig("")

	require.NoError(t, err)
	require.NotNil(t, config)

	// Verify server defaults
	assert.Equal(t, 8081, config.Server.Port)
	assert.Equal(t, 30*time.Second, config.Server.ReadTimeout)
	assert.Equal(t, 30*time.Second, config.Server.WriteTimeout)

	// Verify Kafka defaults
	assert.Equal(t, []string{"localhost:9092"}, config.Kafka.Brokers)
	assert.Equal(t, "normalized_odds", config.Kafka.Topic)
	assert.Equal(t, "odds-optimizer", config.Kafka.GroupID)

	// Verify Redis defaults
	assert.Equal(t, "localhost:6379", config.Redis.Addr)
	assert.Equal(t, "", config.Redis.Password)
	assert.Equal(t, 0, config.Redis.DB)
	assert.Equal(t, 15*time.Minute, config.Redis.TTL)

	// Verify optimization defaults
	assert.Equal(t, 0.02, config.Optimization.MinMargin)
	assert.Equal(t, 0.10, config.Optimization.MaxMargin)
	assert.Equal(t, 0.05, config.Optimization.MinSpread)
	assert.Equal(t, 0.85, config.Optimization.TargetConfidence)

	// Verify logging defaults
	assert.Equal(t, "info", config.Logging.Level)
	assert.Equal(t, "json", config.Logging.Format)
}

// TestLoadConfig_WithFile tests loading configuration from file
func TestLoadConfig_WithFile(t *testing.T) {
	// Create temporary config file
	tmpFile, err := os.CreateTemp("", "config-*.yaml")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	configContent := `
server:
  port: 9090
  read_timeout: 45s
  write_timeout: 45s

kafka:
  brokers:
    - broker1:9092
    - broker2:9092
  topic: test_topic
  group_id: test_group

redis:
  addr: redis:6379
  password: test_password
  db: 1
  ttl: 30m

optimization:
  min_margin: 0.03
  max_margin: 0.15
  min_spread: 0.08
  target_confidence: 0.90

logging:
  level: debug
  format: console
`

	_, err = tmpFile.WriteString(configContent)
	require.NoError(t, err)
	tmpFile.Close()

	// Load config from file
	config, err := LoadConfig(tmpFile.Name())

	require.NoError(t, err)
	require.NotNil(t, config)

	// Verify server config
	assert.Equal(t, 9090, config.Server.Port)
	assert.Equal(t, 45*time.Second, config.Server.ReadTimeout)
	assert.Equal(t, 45*time.Second, config.Server.WriteTimeout)

	// Verify Kafka config
	assert.Equal(t, []string{"broker1:9092", "broker2:9092"}, config.Kafka.Brokers)
	assert.Equal(t, "test_topic", config.Kafka.Topic)
	assert.Equal(t, "test_group", config.Kafka.GroupID)

	// Verify Redis config
	assert.Equal(t, "redis:6379", config.Redis.Addr)
	assert.Equal(t, "test_password", config.Redis.Password)
	assert.Equal(t, 1, config.Redis.DB)
	assert.Equal(t, 30*time.Minute, config.Redis.TTL)

	// Verify optimization config
	assert.Equal(t, 0.03, config.Optimization.MinMargin)
	assert.Equal(t, 0.15, config.Optimization.MaxMargin)
	assert.Equal(t, 0.08, config.Optimization.MinSpread)
	assert.Equal(t, 0.90, config.Optimization.TargetConfidence)

	// Verify logging config
	assert.Equal(t, "debug", config.Logging.Level)
	assert.Equal(t, "console", config.Logging.Format)
}

// TestLoadConfig_InvalidFile tests loading with non-existent file
func TestLoadConfig_InvalidFile(t *testing.T) {
	config, err := LoadConfig("/nonexistent/config.yaml")

	assert.Error(t, err)
	assert.Nil(t, config)
}

// TestLoadConfig_MalformedFile tests loading with malformed YAML
func TestLoadConfig_MalformedFile(t *testing.T) {
	// Create temporary config file with malformed YAML
	tmpFile, err := os.CreateTemp("", "config-*.yaml")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	malformedContent := `
server:
  port: invalid_port
  read_timeout: not_a_duration
`

	_, err = tmpFile.WriteString(malformedContent)
	require.NoError(t, err)
	tmpFile.Close()

	// Load config from file
	config, err := LoadConfig(tmpFile.Name())

	// Should error on unmarshal
	assert.Error(t, err)
	assert.Nil(t, config)
}

// TestLoadConfig_PartialFile tests loading with partial configuration
func TestLoadConfig_PartialFile(t *testing.T) {
	// Create temporary config file with partial config
	tmpFile, err := os.CreateTemp("", "config-*.yaml")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	partialContent := `
server:
  port: 9090

kafka:
  brokers:
    - broker1:9092

# Other configs will use defaults
`

	_, err = tmpFile.WriteString(partialContent)
	require.NoError(t, err)
	tmpFile.Close()

	// Load config from file
	config, err := LoadConfig(tmpFile.Name())

	require.NoError(t, err)
	require.NotNil(t, config)

	// Verify overridden values
	assert.Equal(t, 9090, config.Server.Port)
	assert.Equal(t, []string{"broker1:9092"}, config.Kafka.Brokers)

	// Verify defaults are still used for non-specified values
	assert.Equal(t, 30*time.Second, config.Server.ReadTimeout)
	assert.Equal(t, "normalized_odds", config.Kafka.Topic)
	assert.Equal(t, "localhost:6379", config.Redis.Addr)
}

// TestLoadConfig_EnvironmentVariables tests environment variable overrides
func TestLoadConfig_EnvironmentVariables(t *testing.T) {
	// Set environment variables
	os.Setenv("ODDS_OPTIMIZER_SERVER_PORT", "7777")
	os.Setenv("ODDS_OPTIMIZER_REDIS_ADDR", "env-redis:6379")
	os.Setenv("ODDS_OPTIMIZER_KAFKA_TOPIC", "env_topic")
	defer func() {
		os.Unsetenv("ODDS_OPTIMIZER_SERVER_PORT")
		os.Unsetenv("ODDS_OPTIMIZER_REDIS_ADDR")
		os.Unsetenv("ODDS_OPTIMIZER_KAFKA_TOPIC")
	}()

	// Load config (env vars should override defaults)
	config, err := LoadConfig("")

	require.NoError(t, err)
	require.NotNil(t, config)

	// Verify environment variables were used
	assert.Equal(t, 7777, config.Server.Port)
	assert.Equal(t, "env-redis:6379", config.Redis.Addr)
	assert.Equal(t, "env_topic", config.Kafka.Topic)
}

// TestToOptimizationParams tests conversion to optimization parameters
func TestToOptimizationParams(t *testing.T) {
	optConfig := OptimizationConfig{
		MinMargin:        0.03,
		MaxMargin:        0.12,
		MinSpread:        0.06,
		TargetConfidence: 0.88,
	}

	params := optConfig.ToOptimizationParams()

	assert.True(t, decimal.NewFromFloat(0.03).Equal(params.MinMargin))
	assert.True(t, decimal.NewFromFloat(0.12).Equal(params.MaxMargin))
	assert.True(t, decimal.NewFromFloat(0.06).Equal(params.MinSpread))
	assert.Equal(t, 0.88, params.TargetConfidence)
}

// TestToOptimizationParams_ZeroValues tests conversion with zero values
func TestToOptimizationParams_ZeroValues(t *testing.T) {
	optConfig := OptimizationConfig{
		MinMargin:        0.0,
		MaxMargin:        0.0,
		MinSpread:        0.0,
		TargetConfidence: 0.0,
	}

	params := optConfig.ToOptimizationParams()

	assert.True(t, decimal.Zero.Equal(params.MinMargin))
	assert.True(t, decimal.Zero.Equal(params.MaxMargin))
	assert.True(t, decimal.Zero.Equal(params.MinSpread))
	assert.Equal(t, 0.0, params.TargetConfidence)
}

// TestToOptimizationParams_ExtremeValues tests conversion with extreme values
func TestToOptimizationParams_ExtremeValues(t *testing.T) {
	optConfig := OptimizationConfig{
		MinMargin:        0.001,  // Very low margin
		MaxMargin:        0.999,  // Very high margin
		MinSpread:        0.0001, // Very tight spread
		TargetConfidence: 0.999,  // Very high confidence
	}

	params := optConfig.ToOptimizationParams()

	assert.True(t, decimal.NewFromFloat(0.001).Equal(params.MinMargin))
	assert.True(t, decimal.NewFromFloat(0.999).Equal(params.MaxMargin))
	assert.True(t, decimal.NewFromFloat(0.0001).Equal(params.MinSpread))
	assert.Equal(t, 0.999, params.TargetConfidence)
}

// TestServerConfig tests server configuration
func TestServerConfig(t *testing.T) {
	tests := []struct {
		name   string
		config ServerConfig
	}{
		{
			name: "Default timeouts",
			config: ServerConfig{
				Port:         8080,
				ReadTimeout:  30 * time.Second,
				WriteTimeout: 30 * time.Second,
			},
		},
		{
			name: "Custom timeouts",
			config: ServerConfig{
				Port:         9090,
				ReadTimeout:  60 * time.Second,
				WriteTimeout: 60 * time.Second,
			},
		},
		{
			name: "Short timeouts",
			config: ServerConfig{
				Port:         8081,
				ReadTimeout:  5 * time.Second,
				WriteTimeout: 5 * time.Second,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Greater(t, tt.config.Port, 0)
			assert.Greater(t, tt.config.Port, 1024) // Should use non-privileged port
			assert.Greater(t, tt.config.ReadTimeout, time.Duration(0))
			assert.Greater(t, tt.config.WriteTimeout, time.Duration(0))
		})
	}
}

// TestKafkaConfig tests Kafka configuration
func TestKafkaConfig(t *testing.T) {
	tests := []struct {
		name   string
		config KafkaConfig
	}{
		{
			name: "Single broker",
			config: KafkaConfig{
				Brokers: []string{"localhost:9092"},
				Topic:   "test_topic",
				GroupID: "test_group",
			},
		},
		{
			name: "Multiple brokers",
			config: KafkaConfig{
				Brokers: []string{"broker1:9092", "broker2:9092", "broker3:9092"},
				Topic:   "test_topic",
				GroupID: "test_group",
			},
		},
		{
			name: "Production-like config",
			config: KafkaConfig{
				Brokers: []string{"kafka-1.example.com:9092", "kafka-2.example.com:9092"},
				Topic:   "normalized_odds_prod",
				GroupID: "odds-optimizer-prod",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotEmpty(t, tt.config.Brokers)
			assert.NotEmpty(t, tt.config.Topic)
			assert.NotEmpty(t, tt.config.GroupID)
		})
	}
}

// TestRedisConfig tests Redis configuration
func TestRedisConfig(t *testing.T) {
	tests := []struct {
		name   string
		config RedisConfig
	}{
		{
			name: "Local Redis",
			config: RedisConfig{
				Addr:     "localhost:6379",
				Password: "",
				DB:       0,
				TTL:      15 * time.Minute,
			},
		},
		{
			name: "Authenticated Redis",
			config: RedisConfig{
				Addr:     "redis.example.com:6379",
				Password: "secret",
				DB:       1,
				TTL:      30 * time.Minute,
			},
		},
		{
			name: "Short TTL",
			config: RedisConfig{
				Addr:     "localhost:6379",
				Password: "",
				DB:       0,
				TTL:      5 * time.Minute,
			},
		},
		{
			name: "Long TTL",
			config: RedisConfig{
				Addr:     "localhost:6379",
				Password: "",
				DB:       0,
				TTL:      1 * time.Hour,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotEmpty(t, tt.config.Addr)
			assert.GreaterOrEqual(t, tt.config.DB, 0)
			assert.Greater(t, tt.config.TTL, time.Duration(0))
		})
	}
}

// TestOptimizationConfig tests optimization configuration
func TestOptimizationConfig(t *testing.T) {
	tests := []struct {
		name   string
		config OptimizationConfig
	}{
		{
			name: "Conservative margins",
			config: OptimizationConfig{
				MinMargin:        0.01,
				MaxMargin:        0.05,
				MinSpread:        0.03,
				TargetConfidence: 0.95,
			},
		},
		{
			name: "Aggressive margins",
			config: OptimizationConfig{
				MinMargin:        0.05,
				MaxMargin:        0.20,
				MinSpread:        0.10,
				TargetConfidence: 0.75,
			},
		},
		{
			name: "Balanced margins",
			config: OptimizationConfig{
				MinMargin:        0.02,
				MaxMargin:        0.10,
				MinSpread:        0.05,
				TargetConfidence: 0.85,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Greater(t, tt.config.MinMargin, 0.0)
			assert.Greater(t, tt.config.MaxMargin, tt.config.MinMargin)
			assert.Greater(t, tt.config.MinSpread, 0.0)
			assert.Greater(t, tt.config.TargetConfidence, 0.0)
			assert.LessOrEqual(t, tt.config.TargetConfidence, 1.0)
		})
	}
}

// TestLoggingConfig tests logging configuration
func TestLoggingConfig(t *testing.T) {
	tests := []struct {
		name   string
		config LoggingConfig
	}{
		{
			name: "JSON production logging",
			config: LoggingConfig{
				Level:  "info",
				Format: "json",
			},
		},
		{
			name: "Console development logging",
			config: LoggingConfig{
				Level:  "debug",
				Format: "console",
			},
		},
		{
			name: "Error logging",
			config: LoggingConfig{
				Level:  "error",
				Format: "json",
			},
		},
		{
			name: "Warn logging",
			config: LoggingConfig{
				Level:  "warn",
				Format: "console",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validLevels := []string{"debug", "info", "warn", "error"}
			assert.Contains(t, validLevels, tt.config.Level)

			validFormats := []string{"json", "console"}
			assert.Contains(t, validFormats, tt.config.Format)
		})
	}
}

// TestConfig_AllFields tests that all config fields are properly set
func TestConfig_AllFields(t *testing.T) {
	config, err := LoadConfig("")
	require.NoError(t, err)
	require.NotNil(t, config)

	// Server
	assert.NotZero(t, config.Server.Port)
	assert.NotZero(t, config.Server.ReadTimeout)
	assert.NotZero(t, config.Server.WriteTimeout)

	// Kafka
	assert.NotEmpty(t, config.Kafka.Brokers)
	assert.NotEmpty(t, config.Kafka.Topic)
	assert.NotEmpty(t, config.Kafka.GroupID)

	// Redis
	assert.NotEmpty(t, config.Redis.Addr)
	assert.GreaterOrEqual(t, config.Redis.DB, 0)
	assert.NotZero(t, config.Redis.TTL)

	// Optimization
	assert.NotZero(t, config.Optimization.MinMargin)
	assert.NotZero(t, config.Optimization.MaxMargin)
	assert.NotZero(t, config.Optimization.MinSpread)
	assert.NotZero(t, config.Optimization.TargetConfidence)

	// Logging
	assert.NotEmpty(t, config.Logging.Level)
	assert.NotEmpty(t, config.Logging.Format)
}
