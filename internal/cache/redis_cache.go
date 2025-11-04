package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"

	"github.com/cypherlabdev/odds-optimizer-service/internal/models"
)

// RedisCache caches optimized odds in Redis
type RedisCache struct {
	client *redis.Client
	ttl    time.Duration
	logger zerolog.Logger
}

// RedisCacheConfig holds Redis cache configuration
type RedisCacheConfig struct {
	Addr     string        // e.g., "localhost:6379"
	Password string
	DB       int
	TTL      time.Duration // e.g., 15 * time.Minute
}

// NewRedisCache creates a new Redis cache
func NewRedisCache(config RedisCacheConfig, logger zerolog.Logger) *RedisCache {
	client := redis.NewClient(&redis.Options{
		Addr:     config.Addr,
		Password: config.Password,
		DB:       config.DB,
	})

	return &RedisCache{
		client: client,
		ttl:    config.TTL,
		logger: logger.With().Str("component", "redis_cache").Logger(),
	}
}

// Set caches optimized odds
func (c *RedisCache) Set(ctx context.Context, odds *models.OptimizedOdds) error {
	// Create Redis key: odds:{event_id}:{market}:{selection}
	key := fmt.Sprintf("odds:%s:%s:%s", odds.EventID, odds.Market, odds.Selection)

	// Serialize to JSON
	data, err := json.Marshal(odds)
	if err != nil {
		return fmt.Errorf("failed to marshal odds: %w", err)
	}

	// Set in Redis with TTL
	if err := c.client.Set(ctx, key, data, c.ttl).Err(); err != nil {
		return fmt.Errorf("failed to set in Redis: %w", err)
	}

	c.logger.Debug().
		Str("key", key).
		Dur("ttl", c.ttl).
		Msg("cached optimized odds")

	return nil
}

// Get retrieves cached optimized odds
func (c *RedisCache) Get(ctx context.Context, eventID, market, selection string) (*models.OptimizedOdds, error) {
	key := fmt.Sprintf("odds:%s:%s:%s", eventID, market, selection)

	// Get from Redis
	data, err := c.client.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return nil, fmt.Errorf("odds not found in cache")
	} else if err != nil {
		return nil, fmt.Errorf("failed to get from Redis: %w", err)
	}

	// Deserialize
	var odds models.OptimizedOdds
	if err := json.Unmarshal(data, &odds); err != nil {
		return nil, fmt.Errorf("failed to unmarshal odds: %w", err)
	}

	return &odds, nil
}

// SetBatch caches multiple optimized odds
func (c *RedisCache) SetBatch(ctx context.Context, oddsList []*models.OptimizedOdds) error {
	if len(oddsList) == 0 {
		return nil
	}

	// Use pipeline for batch operations
	pipe := c.client.Pipeline()

	for _, odds := range oddsList {
		key := fmt.Sprintf("odds:%s:%s:%s", odds.EventID, odds.Market, odds.Selection)
		data, err := json.Marshal(odds)
		if err != nil {
			c.logger.Error().Err(err).Msg("failed to marshal odds")
			continue
		}
		pipe.Set(ctx, key, data, c.ttl)
	}

	// Execute pipeline
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("failed to execute pipeline: %w", err)
	}

	c.logger.Info().
		Int("count", len(oddsList)).
		Msg("cached batch of optimized odds")

	return nil
}

// GetByEvent retrieves all cached odds for an event
func (c *RedisCache) GetByEvent(ctx context.Context, eventID string) ([]*models.OptimizedOdds, error) {
	pattern := fmt.Sprintf("odds:%s:*", eventID)

	// Scan for keys matching pattern
	var cursor uint64
	var keys []string

	for {
		var scanKeys []string
		var err error
		scanKeys, cursor, err = c.client.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return nil, fmt.Errorf("failed to scan keys: %w", err)
		}

		keys = append(keys, scanKeys...)

		if cursor == 0 {
			break
		}
	}

	// Get all values
	oddsList := make([]*models.OptimizedOdds, 0, len(keys))
	for _, key := range keys {
		data, err := c.client.Get(ctx, key).Bytes()
		if err != nil {
			c.logger.Warn().Err(err).Str("key", key).Msg("failed to get key")
			continue
		}

		var odds models.OptimizedOdds
		if err := json.Unmarshal(data, &odds); err != nil {
			c.logger.Warn().Err(err).Str("key", key).Msg("failed to unmarshal odds")
			continue
		}

		oddsList = append(oddsList, &odds)
	}

	return oddsList, nil
}

// Ping checks Redis connection
func (c *RedisCache) Ping(ctx context.Context) error {
	return c.client.Ping(ctx).Err()
}

// Close closes the Redis connection
func (c *RedisCache) Close() error {
	return c.client.Close()
}
