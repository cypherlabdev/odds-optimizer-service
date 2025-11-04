package messaging

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/rs/zerolog"
	"github.com/segmentio/kafka-go"

	"github.com/cypherlabdev/odds-optimizer-service/internal/models"
	"github.com/cypherlabdev/odds-optimizer-service/internal/service"
)

// KafkaConsumer consumes normalized odds from Kafka and optimizes them
type KafkaConsumer struct {
	reader    *kafka.Reader
	optimizer service.Optimizer
	cache     service.Cache
	logger    zerolog.Logger
}

// KafkaConsumerConfig holds Kafka consumer configuration
type KafkaConsumerConfig struct {
	Brokers []string // e.g., ["localhost:9092"]
	Topic   string   // e.g., "normalized_odds"
	GroupID string   // e.g., "odds-optimizer"
}

// NewKafkaConsumer creates a new Kafka consumer
func NewKafkaConsumer(
	config KafkaConsumerConfig,
	opt service.Optimizer,
	cache service.Cache,
	logger zerolog.Logger,
) *KafkaConsumer {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:        config.Brokers,
		Topic:          config.Topic,
		GroupID:        config.GroupID,
		MinBytes:       1e3,  // 1KB
		MaxBytes:       10e6, // 10MB
		CommitInterval: 1000, // Commit every 1 second
	})

	return &KafkaConsumer{
		reader:    reader,
		optimizer: opt,
		cache:     cache,
		logger:    logger.With().Str("component", "kafka_consumer").Logger(),
	}
}

// Start begins consuming messages from Kafka
func (c *KafkaConsumer) Start(ctx context.Context) error {
	c.logger.Info().
		Str("topic", c.reader.Config().Topic).
		Str("group_id", c.reader.Config().GroupID).
		Msg("started consuming from Kafka")

	for {
		select {
		case <-ctx.Done():
			c.logger.Info().Msg("stopping Kafka consumer")
			return c.reader.Close()

		default:
			// Read message
			msg, err := c.reader.FetchMessage(ctx)
			if err != nil {
				if err == context.Canceled {
					return nil
				}
				c.logger.Error().Err(err).Msg("failed to fetch message")
				continue
			}

			// Process message
			if err := c.processMessage(ctx, msg); err != nil {
				c.logger.Error().
					Err(err).
					Int64("offset", msg.Offset).
					Str("key", string(msg.Key)).
					Msg("failed to process message")
				// Don't commit if processing failed
				continue
			}

			// Commit message
			if err := c.reader.CommitMessages(ctx, msg); err != nil {
				c.logger.Error().Err(err).Msg("failed to commit message")
			}
		}
	}
}

// processMessage processes a single Kafka message
func (c *KafkaConsumer) processMessage(ctx context.Context, msg kafka.Message) error {
	// Parse message
	var kafkaMsg models.KafkaNormalizedOddsMessage
	if err := json.Unmarshal(msg.Value, &kafkaMsg); err != nil {
		return fmt.Errorf("failed to unmarshal message: %w", err)
	}

	c.logger.Debug().
		Int("odds_count", len(kafkaMsg.OddsData)).
		Str("batch_id", kafkaMsg.BatchID).
		Msg("processing normalized odds batch")

	// Convert to pointers
	normalizedOdds := make([]*models.NormalizedOdds, len(kafkaMsg.OddsData))
	for i := range kafkaMsg.OddsData {
		normalizedOdds[i] = &kafkaMsg.OddsData[i]
	}

	// Optimize odds
	optimizedOdds, err := c.optimizer.BatchOptimize(normalizedOdds)
	if err != nil {
		return fmt.Errorf("failed to optimize odds: %w", err)
	}

	// Cache optimized odds in Redis
	if err := c.cache.SetBatch(ctx, optimizedOdds); err != nil {
		return fmt.Errorf("failed to cache odds: %w", err)
	}

	c.logger.Info().
		Int("input_count", len(normalizedOdds)).
		Int("output_count", len(optimizedOdds)).
		Str("batch_id", kafkaMsg.BatchID).
		Msg("processed and cached optimized odds")

	return nil
}

// Close closes the Kafka reader
func (c *KafkaConsumer) Close() error {
	return c.reader.Close()
}
