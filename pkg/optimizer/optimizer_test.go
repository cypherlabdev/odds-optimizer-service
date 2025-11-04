package optimizer

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cypherlabdev/odds-optimizer-service/internal/models"
)

// testOptimizerSetup is a helper struct to hold test dependencies
type testOptimizerSetup struct {
	optimizer *Optimizer
	params    models.OptimizationParams
}

// setupTestOptimizer creates a test optimizer with default parameters
func setupTestOptimizer() *testOptimizerSetup {
	params := models.OptimizationParams{
		MinMargin:        decimal.NewFromFloat(0.02), // 2%
		MaxMargin:        decimal.NewFromFloat(0.10), // 10%
		MinSpread:        decimal.NewFromFloat(0.05), // 5%
		TargetConfidence: 0.85,
	}

	logger := zerolog.Nop()
	optimizer := NewOptimizer(params, logger)

	return &testOptimizerSetup{
		optimizer: optimizer,
		params:    params,
	}
}

// TestNewOptimizer tests optimizer creation
func TestNewOptimizer(t *testing.T) {
	setup := setupTestOptimizer()
	assert.NotNil(t, setup.optimizer)
	assert.Equal(t, setup.params, setup.optimizer.params)
}

// TestOptimize_Success tests successful odds optimization
func TestOptimize_Success(t *testing.T) {
	setup := setupTestOptimizer()

	normalized := &models.NormalizedOdds{
		ID:          uuid.New(),
		EventID:     "event-123",
		EventName:   "Team A vs Team B",
		Sport:       "football",
		Competition: "Premier League",
		Market:      "match_winner",
		Selection:   "Team A",
		BackPrice:   decimal.NewFromFloat(2.50),
		LayPrice:    decimal.NewFromFloat(2.60),
		BackSize:    decimal.NewFromFloat(10000),
		LaySize:     decimal.NewFromFloat(8000),
		Timestamp:   time.Now(),
	}

	optimized, err := setup.optimizer.Optimize(normalized)

	assert.NoError(t, err)
	assert.NotNil(t, optimized)
	assert.Equal(t, normalized.EventID, optimized.EventID)
	assert.Equal(t, normalized.EventName, optimized.EventName)
	assert.Equal(t, normalized.Sport, optimized.Sport)
	assert.Equal(t, normalized.Market, optimized.Market)
	assert.Equal(t, normalized.Selection, optimized.Selection)
	assert.True(t, optimized.OptimizedBack.GreaterThan(decimal.Zero))
	assert.True(t, optimized.OptimizedLay.GreaterThan(decimal.Zero))
	assert.True(t, optimized.Margin.GreaterThanOrEqual(setup.params.MinMargin))
	assert.True(t, optimized.Margin.LessThanOrEqual(setup.params.MaxMargin))
	assert.True(t, optimized.Confidence > 0 && optimized.Confidence <= 1)
	assert.Equal(t, normalized.BackPrice, optimized.OriginalBack)
	assert.Equal(t, normalized.LayPrice, optimized.OriginalLay)
}

// TestOptimize_InvalidBackPrice tests optimization with invalid back price
func TestOptimize_InvalidBackPrice(t *testing.T) {
	setup := setupTestOptimizer()

	normalized := &models.NormalizedOdds{
		ID:          uuid.New(),
		EventID:     "event-123",
		EventName:   "Team A vs Team B",
		Sport:       "football",
		Competition: "Premier League",
		Market:      "match_winner",
		Selection:   "Team A",
		BackPrice:   decimal.NewFromFloat(0.50), // Invalid: < 1
		LayPrice:    decimal.NewFromFloat(2.60),
		BackSize:    decimal.NewFromFloat(10000),
		LaySize:     decimal.NewFromFloat(8000),
		Timestamp:   time.Now(),
	}

	optimized, err := setup.optimizer.Optimize(normalized)

	assert.Error(t, err)
	assert.Nil(t, optimized)
	assert.Contains(t, err.Error(), "invalid back price")
}

// TestOptimize_ZeroBackPrice tests optimization with zero back price
func TestOptimize_ZeroBackPrice(t *testing.T) {
	setup := setupTestOptimizer()

	normalized := &models.NormalizedOdds{
		ID:          uuid.New(),
		EventID:     "event-123",
		EventName:   "Team A vs Team B",
		Sport:       "football",
		Competition: "Premier League",
		Market:      "match_winner",
		Selection:   "Team A",
		BackPrice:   decimal.Zero,
		LayPrice:    decimal.NewFromFloat(2.60),
		BackSize:    decimal.NewFromFloat(10000),
		LaySize:     decimal.NewFromFloat(8000),
		Timestamp:   time.Now(),
	}

	optimized, err := setup.optimizer.Optimize(normalized)

	assert.Error(t, err)
	assert.Nil(t, optimized)
}

// TestOptimize_WithZeroLayPrice tests optimization when lay price is zero
func TestOptimize_WithZeroLayPrice(t *testing.T) {
	setup := setupTestOptimizer()

	normalized := &models.NormalizedOdds{
		ID:          uuid.New(),
		EventID:     "event-123",
		EventName:   "Team A vs Team B",
		Sport:       "football",
		Competition: "Premier League",
		Market:      "match_winner",
		Selection:   "Team A",
		BackPrice:   decimal.NewFromFloat(2.50),
		LayPrice:    decimal.Zero, // No lay price available
		BackSize:    decimal.NewFromFloat(10000),
		LaySize:     decimal.NewFromFloat(8000),
		Timestamp:   time.Now(),
	}

	optimized, err := setup.optimizer.Optimize(normalized)

	assert.NoError(t, err)
	assert.NotNil(t, optimized)
	assert.True(t, optimized.OptimizedBack.GreaterThan(decimal.Zero))
	assert.True(t, optimized.OptimizedLay.GreaterThan(decimal.Zero))
}

// TestOptimize_MinSpreadEnforced tests that minimum spread is enforced
func TestOptimize_MinSpreadEnforced(t *testing.T) {
	setup := setupTestOptimizer()

	// Use very similar back/lay prices
	normalized := &models.NormalizedOdds{
		ID:          uuid.New(),
		EventID:     "event-123",
		EventName:   "Team A vs Team B",
		Sport:       "football",
		Competition: "Premier League",
		Market:      "match_winner",
		Selection:   "Team A",
		BackPrice:   decimal.NewFromFloat(2.50),
		LayPrice:    decimal.NewFromFloat(2.51), // Very tight spread
		BackSize:    decimal.NewFromFloat(10000),
		LaySize:     decimal.NewFromFloat(8000),
		Timestamp:   time.Now(),
	}

	optimized, err := setup.optimizer.Optimize(normalized)

	assert.NoError(t, err)
	assert.NotNil(t, optimized)

	spread := optimized.OptimizedBack.Sub(optimized.OptimizedLay)
	assert.True(t, spread.GreaterThanOrEqual(setup.params.MinSpread),
		"spread %s should be >= min spread %s", spread, setup.params.MinSpread)
}

// TestOptimize_LowLiquidity tests optimization with low liquidity
func TestOptimize_LowLiquidity(t *testing.T) {
	setup := setupTestOptimizer()

	normalized := &models.NormalizedOdds{
		ID:          uuid.New(),
		EventID:     "event-123",
		EventName:   "Team A vs Team B",
		Sport:       "football",
		Competition: "Premier League",
		Market:      "match_winner",
		Selection:   "Team A",
		BackPrice:   decimal.NewFromFloat(2.50),
		LayPrice:    decimal.NewFromFloat(2.60),
		BackSize:    decimal.NewFromFloat(100),  // Low liquidity
		LaySize:     decimal.NewFromFloat(100),  // Low liquidity
		Timestamp:   time.Now(),
	}

	optimized, err := setup.optimizer.Optimize(normalized)

	assert.NoError(t, err)
	assert.NotNil(t, optimized)
	// With low liquidity, margin should be higher (closer to max)
	assert.True(t, optimized.Margin.GreaterThan(setup.params.MinMargin))
}

// TestOptimize_HighLiquidity tests optimization with high liquidity
func TestOptimize_HighLiquidity(t *testing.T) {
	setup := setupTestOptimizer()

	normalized := &models.NormalizedOdds{
		ID:          uuid.New(),
		EventID:     "event-123",
		EventName:   "Team A vs Team B",
		Sport:       "football",
		Competition: "Premier League",
		Market:      "match_winner",
		Selection:   "Team A",
		BackPrice:   decimal.NewFromFloat(2.50),
		LayPrice:    decimal.NewFromFloat(2.60),
		BackSize:    decimal.NewFromFloat(50000), // High liquidity
		LaySize:     decimal.NewFromFloat(50000), // High liquidity
		Timestamp:   time.Now(),
	}

	optimized, err := setup.optimizer.Optimize(normalized)

	assert.NoError(t, err)
	assert.NotNil(t, optimized)
	// With high liquidity, confidence should be higher
	assert.True(t, optimized.Confidence > 0.5)
}

// TestOptimize_DifferentSports tests margin adjustment for different sports
func TestOptimize_DifferentSports(t *testing.T) {
	tests := []struct {
		name  string
		sport string
	}{
		{"Football", "football"},
		{"Soccer", "soccer"},
		{"Tennis", "tennis"},
		{"Basketball", "basketball"},
		{"Cricket", "cricket"},
	}

	setup := setupTestOptimizer()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			normalized := &models.NormalizedOdds{
				ID:          uuid.New(),
				EventID:     "event-123",
				EventName:   "Event Name",
				Sport:       tt.sport,
				Competition: "Test Competition",
				Market:      "match_winner",
				Selection:   "Selection A",
				BackPrice:   decimal.NewFromFloat(2.50),
				LayPrice:    decimal.NewFromFloat(2.60),
				BackSize:    decimal.NewFromFloat(10000),
				LaySize:     decimal.NewFromFloat(8000),
				Timestamp:   time.Now(),
			}

			optimized, err := setup.optimizer.Optimize(normalized)

			assert.NoError(t, err)
			assert.NotNil(t, optimized)
			assert.Equal(t, tt.sport, optimized.Sport)
			assert.True(t, optimized.Margin.GreaterThanOrEqual(setup.params.MinMargin))
			assert.True(t, optimized.Margin.LessThanOrEqual(setup.params.MaxMargin))
		})
	}
}

// TestOptimize_OldData tests confidence reduction with old data
func TestOptimize_OldData(t *testing.T) {
	setup := setupTestOptimizer()

	normalized := &models.NormalizedOdds{
		ID:          uuid.New(),
		EventID:     "event-123",
		EventName:   "Team A vs Team B",
		Sport:       "football",
		Competition: "Premier League",
		Market:      "match_winner",
		Selection:   "Team A",
		BackPrice:   decimal.NewFromFloat(2.50),
		LayPrice:    decimal.NewFromFloat(2.60),
		BackSize:    decimal.NewFromFloat(10000),
		LaySize:     decimal.NewFromFloat(8000),
		Timestamp:   time.Now().Add(-2 * time.Hour), // Old data
	}

	optimized, err := setup.optimizer.Optimize(normalized)

	assert.NoError(t, err)
	assert.NotNil(t, optimized)
	// Confidence should be reduced for old data
	assert.True(t, optimized.Confidence > 0 && optimized.Confidence < 1)
}

// TestOptimize_FreshData tests higher confidence with fresh data
func TestOptimize_FreshData(t *testing.T) {
	setup := setupTestOptimizer()

	normalized := &models.NormalizedOdds{
		ID:          uuid.New(),
		EventID:     "event-123",
		EventName:   "Team A vs Team B",
		Sport:       "football",
		Competition: "Premier League",
		Market:      "match_winner",
		Selection:   "Team A",
		BackPrice:   decimal.NewFromFloat(2.50),
		LayPrice:    decimal.NewFromFloat(2.60),
		BackSize:    decimal.NewFromFloat(10000),
		LaySize:     decimal.NewFromFloat(8000),
		Timestamp:   time.Now(), // Fresh data
	}

	optimized, err := setup.optimizer.Optimize(normalized)

	assert.NoError(t, err)
	assert.NotNil(t, optimized)
	assert.True(t, optimized.Confidence > 0.5)
}

// TestBatchOptimize_Success tests successful batch optimization
func TestBatchOptimize_Success(t *testing.T) {
	setup := setupTestOptimizer()

	normalized := []*models.NormalizedOdds{
		{
			ID:          uuid.New(),
			EventID:     "event-123",
			EventName:   "Team A vs Team B",
			Sport:       "football",
			Competition: "Premier League",
			Market:      "match_winner",
			Selection:   "Team A",
			BackPrice:   decimal.NewFromFloat(2.50),
			LayPrice:    decimal.NewFromFloat(2.60),
			BackSize:    decimal.NewFromFloat(10000),
			LaySize:     decimal.NewFromFloat(8000),
			Timestamp:   time.Now(),
		},
		{
			ID:          uuid.New(),
			EventID:     "event-123",
			EventName:   "Team A vs Team B",
			Sport:       "football",
			Competition: "Premier League",
			Market:      "match_winner",
			Selection:   "Team B",
			BackPrice:   decimal.NewFromFloat(3.20),
			LayPrice:    decimal.NewFromFloat(3.30),
			BackSize:    decimal.NewFromFloat(8000),
			LaySize:     decimal.NewFromFloat(9000),
			Timestamp:   time.Now(),
		},
		{
			ID:          uuid.New(),
			EventID:     "event-456",
			EventName:   "Team C vs Team D",
			Sport:       "tennis",
			Competition: "Wimbledon",
			Market:      "match_winner",
			Selection:   "Team C",
			BackPrice:   decimal.NewFromFloat(1.80),
			LayPrice:    decimal.NewFromFloat(1.85),
			BackSize:    decimal.NewFromFloat(15000),
			LaySize:     decimal.NewFromFloat(14000),
			Timestamp:   time.Now(),
		},
	}

	optimized, err := setup.optimizer.BatchOptimize(normalized)

	assert.NoError(t, err)
	assert.NotNil(t, optimized)
	assert.Equal(t, 3, len(optimized))

	for i, opt := range optimized {
		assert.Equal(t, normalized[i].EventID, opt.EventID)
		assert.Equal(t, normalized[i].Selection, opt.Selection)
		assert.True(t, opt.OptimizedBack.GreaterThan(decimal.Zero))
		assert.True(t, opt.OptimizedLay.GreaterThan(decimal.Zero))
	}
}

// TestBatchOptimize_EmptyBatch tests batch optimization with empty input
func TestBatchOptimize_EmptyBatch(t *testing.T) {
	setup := setupTestOptimizer()

	normalized := []*models.NormalizedOdds{}

	optimized, err := setup.optimizer.BatchOptimize(normalized)

	assert.NoError(t, err)
	assert.NotNil(t, optimized)
	assert.Equal(t, 0, len(optimized))
}

// TestBatchOptimize_PartialFailure tests batch optimization with some invalid odds
func TestBatchOptimize_PartialFailure(t *testing.T) {
	setup := setupTestOptimizer()

	normalized := []*models.NormalizedOdds{
		{
			ID:          uuid.New(),
			EventID:     "event-123",
			EventName:   "Team A vs Team B",
			Sport:       "football",
			Competition: "Premier League",
			Market:      "match_winner",
			Selection:   "Team A",
			BackPrice:   decimal.NewFromFloat(2.50),
			LayPrice:    decimal.NewFromFloat(2.60),
			BackSize:    decimal.NewFromFloat(10000),
			LaySize:     decimal.NewFromFloat(8000),
			Timestamp:   time.Now(),
		},
		{
			ID:          uuid.New(),
			EventID:     "event-456",
			EventName:   "Team C vs Team D",
			Sport:       "tennis",
			Competition: "Wimbledon",
			Market:      "match_winner",
			Selection:   "Team C",
			BackPrice:   decimal.NewFromFloat(0.50), // Invalid
			LayPrice:    decimal.NewFromFloat(1.85),
			BackSize:    decimal.NewFromFloat(15000),
			LaySize:     decimal.NewFromFloat(14000),
			Timestamp:   time.Now(),
		},
		{
			ID:          uuid.New(),
			EventID:     "event-789",
			EventName:   "Team E vs Team F",
			Sport:       "basketball",
			Competition: "NBA",
			Market:      "match_winner",
			Selection:   "Team E",
			BackPrice:   decimal.NewFromFloat(1.90),
			LayPrice:    decimal.NewFromFloat(1.95),
			BackSize:    decimal.NewFromFloat(12000),
			LaySize:     decimal.NewFromFloat(11000),
			Timestamp:   time.Now(),
		},
	}

	optimized, err := setup.optimizer.BatchOptimize(normalized)

	assert.NoError(t, err)
	assert.NotNil(t, optimized)
	// Should return 2 successful optimizations (skipping the invalid one)
	assert.Equal(t, 2, len(optimized))
}

// TestCalculateImpliedProbability tests implied probability calculation
func TestCalculateImpliedProbability(t *testing.T) {
	setup := setupTestOptimizer()

	tests := []struct {
		name        string
		odds        decimal.Decimal
		expectedProb decimal.Decimal
	}{
		{"Odds 2.00", decimal.NewFromFloat(2.00), decimal.NewFromFloat(0.50)},
		{"Odds 2.50", decimal.NewFromFloat(2.50), decimal.NewFromFloat(0.40)},
		{"Odds 4.00", decimal.NewFromFloat(4.00), decimal.NewFromFloat(0.25)},
		{"Odds 1.50", decimal.NewFromFloat(1.50), decimal.NewFromFloat(0.6666666666666667)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prob := setup.optimizer.calculateImpliedProbability(tt.odds)
			// Allow small difference due to decimal precision
			diff := prob.Sub(tt.expectedProb).Abs()
			assert.True(t, diff.LessThan(decimal.NewFromFloat(0.0001)),
				"expected %s, got %s", tt.expectedProb, prob)
		})
	}
}

// TestProbabilityToOdds tests odds calculation from probability
func TestProbabilityToOdds(t *testing.T) {
	setup := setupTestOptimizer()

	tests := []struct {
		name         string
		probability  decimal.Decimal
		expectedOdds decimal.Decimal
	}{
		{"Prob 0.50", decimal.NewFromFloat(0.50), decimal.NewFromFloat(2.00)},
		{"Prob 0.40", decimal.NewFromFloat(0.40), decimal.NewFromFloat(2.50)},
		{"Prob 0.25", decimal.NewFromFloat(0.25), decimal.NewFromFloat(4.00)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			odds := setup.optimizer.probabilityToOdds(tt.probability)
			// Allow small difference due to decimal precision
			diff := odds.Sub(tt.expectedOdds).Abs()
			assert.True(t, diff.LessThan(decimal.NewFromFloat(0.0001)),
				"expected %s, got %s", tt.expectedOdds, odds)
		})
	}
}

// TestProbabilityToOdds_EdgeCases tests edge cases for probability to odds conversion
func TestProbabilityToOdds_EdgeCases(t *testing.T) {
	setup := setupTestOptimizer()

	tests := []struct {
		name        string
		probability decimal.Decimal
	}{
		{"Zero probability", decimal.Zero},
		{"Negative probability", decimal.NewFromFloat(-0.1)},
		{"Probability equals 1", decimal.NewFromInt(1)},
		{"Probability > 1", decimal.NewFromFloat(1.5)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			odds := setup.optimizer.probabilityToOdds(tt.probability)
			// Should return safeguard value (1.0)
			assert.Equal(t, decimal.NewFromInt(1), odds)
		})
	}
}

// TestCalculateTargetMargin tests target margin calculation
func TestCalculateTargetMargin(t *testing.T) {
	setup := setupTestOptimizer()

	normalized := &models.NormalizedOdds{
		ID:          uuid.New(),
		EventID:     "event-123",
		EventName:   "Team A vs Team B",
		Sport:       "football",
		Competition: "Premier League",
		Market:      "match_winner",
		Selection:   "Team A",
		BackPrice:   decimal.NewFromFloat(2.50),
		LayPrice:    decimal.NewFromFloat(2.60),
		BackSize:    decimal.NewFromFloat(10000),
		LaySize:     decimal.NewFromFloat(8000),
		Timestamp:   time.Now(),
	}

	margin := setup.optimizer.calculateTargetMargin(normalized)

	assert.True(t, margin.GreaterThanOrEqual(setup.params.MinMargin))
	assert.True(t, margin.LessThanOrEqual(setup.params.MaxMargin))
}

// TestCalculateConfidence tests confidence calculation
func TestCalculateConfidence(t *testing.T) {
	setup := setupTestOptimizer()

	normalized := &models.NormalizedOdds{
		ID:          uuid.New(),
		EventID:     "event-123",
		EventName:   "Team A vs Team B",
		Sport:       "football",
		Competition: "Premier League",
		Market:      "match_winner",
		Selection:   "Team A",
		BackPrice:   decimal.NewFromFloat(2.50),
		LayPrice:    decimal.NewFromFloat(2.60),
		BackSize:    decimal.NewFromFloat(10000),
		LaySize:     decimal.NewFromFloat(8000),
		Timestamp:   time.Now(),
	}

	spread := decimal.NewFromFloat(0.10)
	confidence := setup.optimizer.calculateConfidence(normalized, spread)

	assert.True(t, confidence >= 0.0 && confidence <= 1.0)
}

// TestOptimize_ConcurrentAccess tests thread safety
func TestOptimize_ConcurrentAccess(t *testing.T) {
	setup := setupTestOptimizer()

	normalized := &models.NormalizedOdds{
		ID:          uuid.New(),
		EventID:     "event-123",
		EventName:   "Team A vs Team B",
		Sport:       "football",
		Competition: "Premier League",
		Market:      "match_winner",
		Selection:   "Team A",
		BackPrice:   decimal.NewFromFloat(2.50),
		LayPrice:    decimal.NewFromFloat(2.60),
		BackSize:    decimal.NewFromFloat(10000),
		LaySize:     decimal.NewFromFloat(8000),
		Timestamp:   time.Now(),
	}

	// Run multiple optimizations concurrently
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			optimized, err := setup.optimizer.Optimize(normalized)
			assert.NoError(t, err)
			assert.NotNil(t, optimized)
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
}

// TestOptimize_PreserveOriginalData tests that original data is preserved
func TestOptimize_PreserveOriginalData(t *testing.T) {
	setup := setupTestOptimizer()

	originalBackPrice := decimal.NewFromFloat(2.50)
	originalLayPrice := decimal.NewFromFloat(2.60)
	originalBackSize := decimal.NewFromFloat(10000)
	originalLaySize := decimal.NewFromFloat(8000)

	normalized := &models.NormalizedOdds{
		ID:          uuid.New(),
		EventID:     "event-123",
		EventName:   "Team A vs Team B",
		Sport:       "football",
		Competition: "Premier League",
		Market:      "match_winner",
		Selection:   "Team A",
		BackPrice:   originalBackPrice,
		LayPrice:    originalLayPrice,
		BackSize:    originalBackSize,
		LaySize:     originalLaySize,
		Timestamp:   time.Now(),
	}

	optimized, err := setup.optimizer.Optimize(normalized)

	require.NoError(t, err)
	require.NotNil(t, optimized)

	// Verify original values are preserved
	assert.Equal(t, originalBackPrice, optimized.OriginalBack)
	assert.Equal(t, originalLayPrice, optimized.OriginalLay)
	assert.Equal(t, originalBackSize, optimized.BackSize)
	assert.Equal(t, originalLaySize, optimized.LaySize)
}
