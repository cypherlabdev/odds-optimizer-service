package service

import (
	"context"
	"fmt"

	"github.com/rs/zerolog"

	"github.com/cypherlabdev/odds-optimizer-service/internal/models"
	"github.com/cypherlabdev/odds-optimizer-service/pkg/optimizer"
)

// OptimizerService orchestrates odds optimization with caching
type OptimizerService struct {
	optimizer *optimizer.Optimizer
	cache     Cache
	logger    zerolog.Logger
}

// NewOptimizerService creates a new optimizer service
func NewOptimizerService(
	optimizer *optimizer.Optimizer,
	cache Cache,
	logger zerolog.Logger,
) *OptimizerService {
	return &OptimizerService{
		optimizer: optimizer,
		cache:     cache,
		logger:    logger.With().Str("component", "optimizer_service").Logger(),
	}
}

// GetOptimizedOdds retrieves optimized odds with cache-first strategy
func (s *OptimizerService) GetOptimizedOdds(ctx context.Context, eventID, market, selection string) (*models.OptimizedOdds, error) {
	// Try cache first
	cached, err := s.cache.Get(ctx, eventID, market, selection)
	if err == nil && cached != nil {
		s.logger.Debug().
			Str("event_id", eventID).
			Str("market", market).
			Str("selection", selection).
			Msg("cache hit for optimized odds")
		return cached, nil
	}

	// Log cache miss (but don't fail on cache errors)
	if err != nil {
		s.logger.Warn().
			Err(err).
			Str("event_id", eventID).
			Str("market", market).
			Str("selection", selection).
			Msg("cache error, will need normalized odds to optimize")
	}

	// Cache miss - caller needs to provide normalized odds to optimize
	return nil, fmt.Errorf("odds not found in cache for event=%s market=%s selection=%s", eventID, market, selection)
}

// OptimizeOdds optimizes normalized odds and caches the result
func (s *OptimizerService) OptimizeOdds(ctx context.Context, normalized *models.NormalizedOdds) (*models.OptimizedOdds, error) {
	// Apply optimization algorithm
	optimized, err := s.optimizer.Optimize(normalized)
	if err != nil {
		return nil, fmt.Errorf("optimization failed: %w", err)
	}

	// Cache the optimized odds
	if err := s.cache.Set(ctx, optimized); err != nil {
		s.logger.Warn().
			Err(err).
			Str("event_id", optimized.EventID).
			Str("market", optimized.Market).
			Str("selection", optimized.Selection).
			Msg("failed to cache optimized odds")
		// Don't fail the request on cache errors
	}

	s.logger.Info().
		Str("event_id", optimized.EventID).
		Str("market", optimized.Market).
		Str("selection", optimized.Selection).
		Str("optimized_back", optimized.OptimizedBack.String()).
		Str("margin", optimized.Margin.String()).
		Float64("confidence", optimized.Confidence).
		Msg("optimized and cached odds")

	return optimized, nil
}

// OptimizeBatch optimizes a batch of normalized odds and caches results
func (s *OptimizerService) OptimizeBatch(ctx context.Context, normalized []*models.NormalizedOdds) ([]*models.OptimizedOdds, error) {
	if len(normalized) == 0 {
		return nil, nil
	}

	// Apply batch optimization
	optimized, err := s.optimizer.BatchOptimize(normalized)
	if err != nil {
		return nil, fmt.Errorf("batch optimization failed: %w", err)
	}

	// Cache all optimized odds in batch
	if err := s.cache.SetBatch(ctx, optimized); err != nil {
		s.logger.Warn().
			Err(err).
			Int("count", len(optimized)).
			Msg("failed to cache batch of optimized odds")
		// Don't fail the request on cache errors
	}

	s.logger.Info().
		Int("input_count", len(normalized)).
		Int("output_count", len(optimized)).
		Msg("optimized and cached batch")

	return optimized, nil
}

// GetOptimizedOddsByEvent retrieves all optimized odds for an event from cache
func (s *OptimizerService) GetOptimizedOddsByEvent(ctx context.Context, eventID string) ([]*models.OptimizedOdds, error) {
	odds, err := s.cache.GetByEvent(ctx, eventID)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve odds for event: %w", err)
	}

	s.logger.Debug().
		Str("event_id", eventID).
		Int("count", len(odds)).
		Msg("retrieved optimized odds by event")

	return odds, nil
}
