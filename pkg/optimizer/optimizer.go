package optimizer

import (
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/shopspring/decimal"

	"github.com/cypherlabdev/odds-optimizer-service/internal/models"
)

// Optimizer applies ML-based optimization to odds
type Optimizer struct {
	params models.OptimizationParams
	logger zerolog.Logger
}

// NewOptimizer creates a new odds optimizer
func NewOptimizer(params models.OptimizationParams, logger zerolog.Logger) *Optimizer {
	return &Optimizer{
		params: params,
		logger: logger.With().Str("component", "optimizer").Logger(),
	}
}

// Optimize applies optimization algorithms to normalized odds
func (o *Optimizer) Optimize(normalized *models.NormalizedOdds) (*models.OptimizedOdds, error) {
	// Validate input
	if normalized.BackPrice.LessThanOrEqual(decimal.NewFromInt(1)) {
		return nil, fmt.Errorf("invalid back price: %s", normalized.BackPrice.String())
	}

	// Calculate implied probability from original odds
	impliedProbBack := o.calculateImpliedProbability(normalized.BackPrice)
	_ = decimal.Zero // impliedProbLay for future use
	if !normalized.LayPrice.IsZero() && normalized.LayPrice.GreaterThan(decimal.NewFromInt(1)) {
		_ = o.calculateImpliedProbability(normalized.LayPrice)
	}

	// Apply margin optimization
	targetMargin := o.calculateTargetMargin(normalized)

	// Calculate optimized probabilities (add our margin)
	optimizedProbBack := impliedProbBack.Add(targetMargin.Div(decimal.NewFromInt(2)))
	optimizedProbLay := impliedProbBack.Sub(targetMargin.Div(decimal.NewFromInt(2)))

	// Convert probabilities back to odds
	optimizedBack := o.probabilityToOdds(optimizedProbBack)
	optimizedLay := o.probabilityToOdds(optimizedProbLay)

	// Ensure minimum spread
	spread := optimizedBack.Sub(optimizedLay)
	if spread.LessThan(o.params.MinSpread) {
		adjustment := o.params.MinSpread.Sub(spread).Div(decimal.NewFromInt(2))
		optimizedBack = optimizedBack.Add(adjustment)
		optimizedLay = optimizedLay.Sub(adjustment)
	}

	// Calculate confidence based on liquidity and spread
	confidence := o.calculateConfidence(normalized, spread)

	return &models.OptimizedOdds{
		ID:              uuid.New(),
		EventID:         normalized.EventID,
		EventName:       normalized.EventName,
		Sport:           normalized.Sport,
		Competition:     normalized.Competition,
		Market:          normalized.Market,
		Selection:       normalized.Selection,
		OptimizedBack:   optimizedBack,
		OptimizedLay:    optimizedLay,
		OriginalBack:    normalized.BackPrice,
		OriginalLay:     normalized.LayPrice,
		BackSize:        normalized.BackSize,
		LaySize:         normalized.LaySize,
		Margin:          targetMargin,
		Confidence:      confidence,
		Timestamp:       normalized.Timestamp,
		OptimizedAt:     time.Now().UTC(),
	}, nil
}

// calculateImpliedProbability converts decimal odds to implied probability
func (o *Optimizer) calculateImpliedProbability(odds decimal.Decimal) decimal.Decimal {
	// Implied probability = 1 / decimal_odds
	// Example: 2.50 odds = 1/2.50 = 0.40 = 40%
	return decimal.NewFromInt(1).Div(odds)
}

// probabilityToOdds converts implied probability to decimal odds
func (o *Optimizer) probabilityToOdds(prob decimal.Decimal) decimal.Decimal {
	// Decimal odds = 1 / probability
	// Example: 40% probability = 1/0.40 = 2.50 odds
	if prob.LessThanOrEqual(decimal.Zero) || prob.GreaterThanOrEqual(decimal.NewFromInt(1)) {
		return decimal.NewFromInt(1) // Safeguard
	}
	return decimal.NewFromInt(1).Div(prob)
}

// calculateTargetMargin determines the optimal margin based on event characteristics
func (o *Optimizer) calculateTargetMargin(normalized *models.NormalizedOdds) decimal.Decimal {
	// Start with base margin
	margin := o.params.MinMargin

	// Adjust margin based on liquidity (lower liquidity = higher margin/risk)
	totalLiquidity := normalized.BackSize.Add(normalized.LaySize)
	liquidityThreshold := decimal.NewFromInt(10000) // $10k threshold

	if totalLiquidity.LessThan(liquidityThreshold) {
		// Low liquidity: increase margin
		liquidityFactor := totalLiquidity.Div(liquidityThreshold)
		marginIncrease := o.params.MaxMargin.Sub(o.params.MinMargin).Mul(decimal.NewFromInt(1).Sub(liquidityFactor))
		margin = margin.Add(marginIncrease)
	}

	// Adjust margin based on sport/market type (could use ML model here)
	// For now, use simple rules:
	switch normalized.Sport {
	case "football", "soccer":
		// Lower margin for high-volume sports
		margin = margin.Mul(decimal.NewFromFloat(0.8))
	case "tennis":
		// Moderate margin
		margin = margin.Mul(decimal.NewFromFloat(1.0))
	default:
		// Higher margin for niche sports
		margin = margin.Mul(decimal.NewFromFloat(1.2))
	}

	// Ensure margin is within bounds
	if margin.LessThan(o.params.MinMargin) {
		margin = o.params.MinMargin
	}
	if margin.GreaterThan(o.params.MaxMargin) {
		margin = o.params.MaxMargin
	}

	return margin
}

// calculateConfidence calculates model confidence based on various factors
func (o *Optimizer) calculateConfidence(normalized *models.NormalizedOdds, spread decimal.Decimal) float64 {
	// Base confidence
	confidence := o.params.TargetConfidence

	// Factor 1: Liquidity (more liquidity = higher confidence)
	totalLiquidity := normalized.BackSize.Add(normalized.LaySize)
	liquidityScore := math.Min(1.0, totalLiquidity.InexactFloat64()/20000.0) // Max at $20k
	confidence *= (0.7 + 0.3*liquidityScore) // Scale 0.7-1.0

	// Factor 2: Spread (tighter spread = higher confidence)
	spreadPercent := spread.Div(normalized.BackPrice).InexactFloat64()
	spreadScore := math.Max(0.0, 1.0-spreadPercent*10) // Penalty for wide spreads
	confidence *= (0.8 + 0.2*spreadScore) // Scale 0.8-1.0

	// Factor 3: Data freshness (newer = higher confidence)
	age := time.Since(normalized.Timestamp)
	freshnessScore := math.Max(0.0, 1.0-age.Minutes()/60.0) // Decay over 1 hour
	confidence *= (0.9 + 0.1*freshnessScore) // Scale 0.9-1.0

	// Clamp confidence to [0, 1]
	if confidence < 0.0 {
		confidence = 0.0
	}
	if confidence > 1.0 {
		confidence = 1.0
	}

	return confidence
}

// BatchOptimize optimizes a batch of normalized odds
func (o *Optimizer) BatchOptimize(normalized []*models.NormalizedOdds) ([]*models.OptimizedOdds, error) {
	optimized := make([]*models.OptimizedOdds, 0, len(normalized))

	for _, odds := range normalized {
		opt, err := o.Optimize(odds)
		if err != nil {
			o.logger.Warn().
				Err(err).
				Str("event_id", odds.EventID).
				Str("selection", odds.Selection).
				Msg("failed to optimize odds")
			continue
		}
		optimized = append(optimized, opt)
	}

	o.logger.Info().
		Int("input_count", len(normalized)).
		Int("output_count", len(optimized)).
		Msg("batch optimization complete")

	return optimized, nil
}
