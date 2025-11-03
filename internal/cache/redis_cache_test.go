package cache

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cypherlabdev/odds-optimizer-service/internal/models"
)

// testRedisCacheSetup is a helper struct to hold test dependencies
type testRedisCacheSetup struct {
	cache      *RedisCache
	miniRedis  *miniredis.Miniredis
	ctx        context.Context
}

// setupTestRedisCache creates a test cache with miniredis
func setupTestRedisCache(t *testing.T) *testRedisCacheSetup {
	// Create miniredis server
	mr, err := miniredis.Run()
	require.NoError(t, err)

	logger := zerolog.Nop()

	config := RedisCacheConfig{
		Addr:     mr.Addr(),
		Password: "",
		DB:       0,
		TTL:      15 * time.Minute,
	}

	cache := NewRedisCache(config, logger)
	ctx := context.Background()

	return &testRedisCacheSetup{
		cache:     cache,
		miniRedis: mr,
		ctx:       ctx,
	}
}

// cleanup cleans up test resources
func (s *testRedisCacheSetup) cleanup() {
	s.cache.Close()
	s.miniRedis.Close()
}

// TestNewRedisCache tests cache creation
func TestNewRedisCache(t *testing.T) {
	setup := setupTestRedisCache(t)
	defer setup.cleanup()

	assert.NotNil(t, setup.cache)
	assert.NotNil(t, setup.cache.client)
	assert.Equal(t, 15*time.Minute, setup.cache.ttl)
}

// TestSet_Success tests successful odds caching
func TestSet_Success(t *testing.T) {
	setup := setupTestRedisCache(t)
	defer setup.cleanup()

	odds := &models.OptimizedOdds{
		ID:            uuid.New(),
		EventID:       "event-123",
		EventName:     "Team A vs Team B",
		Sport:         "football",
		Competition:   "Premier League",
		Market:        "match_winner",
		Selection:     "Team A",
		OptimizedBack: decimal.NewFromFloat(2.45),
		OptimizedLay:  decimal.NewFromFloat(2.55),
		OriginalBack:  decimal.NewFromFloat(2.50),
		OriginalLay:   decimal.NewFromFloat(2.60),
		BackSize:      decimal.NewFromFloat(10000),
		LaySize:       decimal.NewFromFloat(8000),
		Margin:        decimal.NewFromFloat(0.02),
		Confidence:    0.85,
		Timestamp:     time.Now(),
		OptimizedAt:   time.Now(),
	}

	err := setup.cache.Set(setup.ctx, odds)

	assert.NoError(t, err)

	// Verify data was cached
	key := "odds:event-123:match_winner:Team A"
	exists := setup.miniRedis.Exists(key)
	assert.True(t, exists)
}

// TestSet_ContextCanceled tests set operation with canceled context
func TestSet_ContextCanceled(t *testing.T) {
	setup := setupTestRedisCache(t)
	defer setup.cleanup()

	odds := &models.OptimizedOdds{
		ID:            uuid.New(),
		EventID:       "event-123",
		EventName:     "Team A vs Team B",
		Sport:         "football",
		Market:        "match_winner",
		Selection:     "Team A",
		OptimizedBack: decimal.NewFromFloat(2.45),
		OptimizedLay:  decimal.NewFromFloat(2.55),
		Timestamp:     time.Now(),
		OptimizedAt:   time.Now(),
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err := setup.cache.Set(ctx, odds)

	assert.Error(t, err)
}

// TestGet_Success tests successful odds retrieval
func TestGet_Success(t *testing.T) {
	setup := setupTestRedisCache(t)
	defer setup.cleanup()

	originalOdds := &models.OptimizedOdds{
		ID:            uuid.New(),
		EventID:       "event-123",
		EventName:     "Team A vs Team B",
		Sport:         "football",
		Competition:   "Premier League",
		Market:        "match_winner",
		Selection:     "Team A",
		OptimizedBack: decimal.NewFromFloat(2.45),
		OptimizedLay:  decimal.NewFromFloat(2.55),
		OriginalBack:  decimal.NewFromFloat(2.50),
		OriginalLay:   decimal.NewFromFloat(2.60),
		BackSize:      decimal.NewFromFloat(10000),
		LaySize:       decimal.NewFromFloat(8000),
		Margin:        decimal.NewFromFloat(0.02),
		Confidence:    0.85,
		Timestamp:     time.Now().Truncate(time.Second),
		OptimizedAt:   time.Now().Truncate(time.Second),
	}

	// First, cache the odds
	err := setup.cache.Set(setup.ctx, originalOdds)
	require.NoError(t, err)

	// Then retrieve it
	retrievedOdds, err := setup.cache.Get(setup.ctx, "event-123", "match_winner", "Team A")

	assert.NoError(t, err)
	assert.NotNil(t, retrievedOdds)
	assert.Equal(t, originalOdds.EventID, retrievedOdds.EventID)
	assert.Equal(t, originalOdds.Market, retrievedOdds.Market)
	assert.Equal(t, originalOdds.Selection, retrievedOdds.Selection)
	assert.True(t, originalOdds.OptimizedBack.Equal(retrievedOdds.OptimizedBack))
	assert.True(t, originalOdds.OptimizedLay.Equal(retrievedOdds.OptimizedLay))
}

// TestGet_NotFound tests retrieval when odds don't exist
func TestGet_NotFound(t *testing.T) {
	setup := setupTestRedisCache(t)
	defer setup.cleanup()

	retrievedOdds, err := setup.cache.Get(setup.ctx, "nonexistent", "market", "selection")

	assert.Error(t, err)
	assert.Nil(t, retrievedOdds)
	assert.Contains(t, err.Error(), "not found in cache")
}

// TestGet_ExpiredKey tests retrieval of expired key
func TestGet_ExpiredKey(t *testing.T) {
	setup := setupTestRedisCache(t)
	defer setup.cleanup()

	odds := &models.OptimizedOdds{
		ID:            uuid.New(),
		EventID:       "event-123",
		EventName:     "Team A vs Team B",
		Sport:         "football",
		Market:        "match_winner",
		Selection:     "Team A",
		OptimizedBack: decimal.NewFromFloat(2.45),
		OptimizedLay:  decimal.NewFromFloat(2.55),
		Timestamp:     time.Now(),
		OptimizedAt:   time.Now(),
	}

	// Cache the odds
	err := setup.cache.Set(setup.ctx, odds)
	require.NoError(t, err)

	// Fast forward time to expire the key
	setup.miniRedis.FastForward(20 * time.Minute)

	// Try to retrieve expired odds
	retrievedOdds, err := setup.cache.Get(setup.ctx, "event-123", "match_winner", "Team A")

	assert.Error(t, err)
	assert.Nil(t, retrievedOdds)
}

// TestSetBatch_Success tests successful batch caching
func TestSetBatch_Success(t *testing.T) {
	setup := setupTestRedisCache(t)
	defer setup.cleanup()

	oddsList := []*models.OptimizedOdds{
		{
			ID:            uuid.New(),
			EventID:       "event-123",
			EventName:     "Team A vs Team B",
			Sport:         "football",
			Market:        "match_winner",
			Selection:     "Team A",
			OptimizedBack: decimal.NewFromFloat(2.45),
			OptimizedLay:  decimal.NewFromFloat(2.55),
			Timestamp:     time.Now(),
			OptimizedAt:   time.Now(),
		},
		{
			ID:            uuid.New(),
			EventID:       "event-123",
			EventName:     "Team A vs Team B",
			Sport:         "football",
			Market:        "match_winner",
			Selection:     "Team B",
			OptimizedBack: decimal.NewFromFloat(3.15),
			OptimizedLay:  decimal.NewFromFloat(3.25),
			Timestamp:     time.Now(),
			OptimizedAt:   time.Now(),
		},
		{
			ID:            uuid.New(),
			EventID:       "event-456",
			EventName:     "Team C vs Team D",
			Sport:         "tennis",
			Market:        "match_winner",
			Selection:     "Team C",
			OptimizedBack: decimal.NewFromFloat(1.75),
			OptimizedLay:  decimal.NewFromFloat(1.80),
			Timestamp:     time.Now(),
			OptimizedAt:   time.Now(),
		},
	}

	err := setup.cache.SetBatch(setup.ctx, oddsList)

	assert.NoError(t, err)

	// Verify all items were cached
	assert.True(t, setup.miniRedis.Exists("odds:event-123:match_winner:Team A"))
	assert.True(t, setup.miniRedis.Exists("odds:event-123:match_winner:Team B"))
	assert.True(t, setup.miniRedis.Exists("odds:event-456:match_winner:Team C"))
}

// TestSetBatch_EmptyList tests batch caching with empty list
func TestSetBatch_EmptyList(t *testing.T) {
	setup := setupTestRedisCache(t)
	defer setup.cleanup()

	oddsList := []*models.OptimizedOdds{}

	err := setup.cache.SetBatch(setup.ctx, oddsList)

	assert.NoError(t, err)
}

// TestSetBatch_NilList tests batch caching with nil list
func TestSetBatch_NilList(t *testing.T) {
	setup := setupTestRedisCache(t)
	defer setup.cleanup()

	err := setup.cache.SetBatch(setup.ctx, nil)

	assert.NoError(t, err)
}

// TestGetByEvent_Success tests successful retrieval by event
func TestGetByEvent_Success(t *testing.T) {
	setup := setupTestRedisCache(t)
	defer setup.cleanup()

	oddsList := []*models.OptimizedOdds{
		{
			ID:            uuid.New(),
			EventID:       "event-123",
			EventName:     "Team A vs Team B",
			Sport:         "football",
			Market:        "match_winner",
			Selection:     "Team A",
			OptimizedBack: decimal.NewFromFloat(2.45),
			OptimizedLay:  decimal.NewFromFloat(2.55),
			Timestamp:     time.Now(),
			OptimizedAt:   time.Now(),
		},
		{
			ID:            uuid.New(),
			EventID:       "event-123",
			EventName:     "Team A vs Team B",
			Sport:         "football",
			Market:        "match_winner",
			Selection:     "Team B",
			OptimizedBack: decimal.NewFromFloat(3.15),
			OptimizedLay:  decimal.NewFromFloat(3.25),
			Timestamp:     time.Now(),
			OptimizedAt:   time.Now(),
		},
		{
			ID:            uuid.New(),
			EventID:       "event-123",
			EventName:     "Team A vs Team B",
			Sport:         "football",
			Market:        "over_under",
			Selection:     "Over 2.5",
			OptimizedBack: decimal.NewFromFloat(1.90),
			OptimizedLay:  decimal.NewFromFloat(1.95),
			Timestamp:     time.Now(),
			OptimizedAt:   time.Now(),
		},
	}

	// Cache the odds
	err := setup.cache.SetBatch(setup.ctx, oddsList)
	require.NoError(t, err)

	// Retrieve by event
	retrievedOdds, err := setup.cache.GetByEvent(setup.ctx, "event-123")

	assert.NoError(t, err)
	assert.NotNil(t, retrievedOdds)
	assert.Equal(t, 3, len(retrievedOdds))
}

// TestGetByEvent_NotFound tests retrieval by event when no odds exist
func TestGetByEvent_NotFound(t *testing.T) {
	setup := setupTestRedisCache(t)
	defer setup.cleanup()

	retrievedOdds, err := setup.cache.GetByEvent(setup.ctx, "nonexistent-event")

	assert.NoError(t, err)
	assert.NotNil(t, retrievedOdds)
	assert.Equal(t, 0, len(retrievedOdds))
}

// TestGetByEvent_PartialData tests retrieval with some corrupted data
func TestGetByEvent_PartialData(t *testing.T) {
	setup := setupTestRedisCache(t)
	defer setup.cleanup()

	// Cache valid odds
	validOdds := &models.OptimizedOdds{
		ID:            uuid.New(),
		EventID:       "event-123",
		EventName:     "Team A vs Team B",
		Sport:         "football",
		Market:        "match_winner",
		Selection:     "Team A",
		OptimizedBack: decimal.NewFromFloat(2.45),
		OptimizedLay:  decimal.NewFromFloat(2.55),
		Timestamp:     time.Now(),
		OptimizedAt:   time.Now(),
	}

	err := setup.cache.Set(setup.ctx, validOdds)
	require.NoError(t, err)

	// Manually add corrupted data
	setup.miniRedis.Set("odds:event-123:match_winner:Team B", "invalid json data")

	// Retrieve by event - should return only valid odds
	retrievedOdds, err := setup.cache.GetByEvent(setup.ctx, "event-123")

	assert.NoError(t, err)
	assert.NotNil(t, retrievedOdds)
	assert.Equal(t, 1, len(retrievedOdds)) // Only valid odds
}

// TestPing_Success tests successful Redis ping
func TestPing_Success(t *testing.T) {
	setup := setupTestRedisCache(t)
	defer setup.cleanup()

	err := setup.cache.Ping(setup.ctx)

	assert.NoError(t, err)
}

// TestPing_RedisDown tests ping when Redis is down
func TestPing_RedisDown(t *testing.T) {
	setup := setupTestRedisCache(t)

	// Close Redis before ping
	setup.miniRedis.Close()

	err := setup.cache.Ping(setup.ctx)

	assert.Error(t, err)

	// Don't call cleanup() since we already closed Redis
	setup.cache.Close()
}

// TestClose tests cache closing
func TestClose(t *testing.T) {
	setup := setupTestRedisCache(t)
	defer setup.miniRedis.Close()

	err := setup.cache.Close()

	assert.NoError(t, err)
}

// TestSet_DifferentMarkets tests caching odds for different markets
func TestSet_DifferentMarkets(t *testing.T) {
	setup := setupTestRedisCache(t)
	defer setup.cleanup()

	markets := []string{"match_winner", "over_under", "handicap", "correct_score"}

	for _, market := range markets {
		odds := &models.OptimizedOdds{
			ID:            uuid.New(),
			EventID:       "event-123",
			EventName:     "Team A vs Team B",
			Sport:         "football",
			Market:        market,
			Selection:     "Selection",
			OptimizedBack: decimal.NewFromFloat(2.45),
			OptimizedLay:  decimal.NewFromFloat(2.55),
			Timestamp:     time.Now(),
			OptimizedAt:   time.Now(),
		}

		err := setup.cache.Set(setup.ctx, odds)
		assert.NoError(t, err)

		// Verify retrieval
		retrieved, err := setup.cache.Get(setup.ctx, "event-123", market, "Selection")
		assert.NoError(t, err)
		assert.NotNil(t, retrieved)
		assert.Equal(t, market, retrieved.Market)
	}
}

// TestSet_SpecialCharactersInSelection tests caching with special characters
func TestSet_SpecialCharactersInSelection(t *testing.T) {
	setup := setupTestRedisCache(t)
	defer setup.cleanup()

	selections := []string{
		"Team A",
		"Team-B",
		"Team_C",
		"Team:D",
		"Team/E",
		"Team (F)",
	}

	for _, selection := range selections {
		odds := &models.OptimizedOdds{
			ID:            uuid.New(),
			EventID:       "event-123",
			EventName:     "Test Event",
			Sport:         "football",
			Market:        "match_winner",
			Selection:     selection,
			OptimizedBack: decimal.NewFromFloat(2.45),
			OptimizedLay:  decimal.NewFromFloat(2.55),
			Timestamp:     time.Now(),
			OptimizedAt:   time.Now(),
		}

		err := setup.cache.Set(setup.ctx, odds)
		assert.NoError(t, err)

		// Verify retrieval
		retrieved, err := setup.cache.Get(setup.ctx, "event-123", "match_winner", selection)
		assert.NoError(t, err)
		assert.NotNil(t, retrieved)
		assert.Equal(t, selection, retrieved.Selection)
	}
}

// TestSetBatch_LargeBatch tests batch caching with many items
func TestSetBatch_LargeBatch(t *testing.T) {
	setup := setupTestRedisCache(t)
	defer setup.cleanup()

	oddsList := make([]*models.OptimizedOdds, 100)
	for i := 0; i < 100; i++ {
		oddsList[i] = &models.OptimizedOdds{
			ID:            uuid.New(),
			EventID:       "event-123",
			EventName:     "Team A vs Team B",
			Sport:         "football",
			Market:        "match_winner",
			Selection:     "Selection " + string(rune(i)),
			OptimizedBack: decimal.NewFromFloat(2.45),
			OptimizedLay:  decimal.NewFromFloat(2.55),
			Timestamp:     time.Now(),
			OptimizedAt:   time.Now(),
		}
	}

	err := setup.cache.SetBatch(setup.ctx, oddsList)

	assert.NoError(t, err)
}

// TestCache_ConcurrentAccess tests thread safety
func TestCache_ConcurrentAccess(t *testing.T) {
	setup := setupTestRedisCache(t)
	defer setup.cleanup()

	odds := &models.OptimizedOdds{
		ID:            uuid.New(),
		EventID:       "event-123",
		EventName:     "Team A vs Team B",
		Sport:         "football",
		Market:        "match_winner",
		Selection:     "Team A",
		OptimizedBack: decimal.NewFromFloat(2.45),
		OptimizedLay:  decimal.NewFromFloat(2.55),
		Timestamp:     time.Now(),
		OptimizedAt:   time.Now(),
	}

	// Set initial odds
	err := setup.cache.Set(setup.ctx, odds)
	require.NoError(t, err)

	// Run concurrent reads and writes
	done := make(chan bool)

	// Writers
	for i := 0; i < 5; i++ {
		go func() {
			err := setup.cache.Set(setup.ctx, odds)
			assert.NoError(t, err)
			done <- true
		}()
	}

	// Readers
	for i := 0; i < 5; i++ {
		go func() {
			retrieved, err := setup.cache.Get(setup.ctx, "event-123", "match_winner", "Team A")
			assert.NoError(t, err)
			assert.NotNil(t, retrieved)
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
}

// TestCache_TTLRespected tests that TTL is properly set
func TestCache_TTLRespected(t *testing.T) {
	setup := setupTestRedisCache(t)
	defer setup.cleanup()

	odds := &models.OptimizedOdds{
		ID:            uuid.New(),
		EventID:       "event-123",
		EventName:     "Team A vs Team B",
		Sport:         "football",
		Market:        "match_winner",
		Selection:     "Team A",
		OptimizedBack: decimal.NewFromFloat(2.45),
		OptimizedLay:  decimal.NewFromFloat(2.55),
		Timestamp:     time.Now(),
		OptimizedAt:   time.Now(),
	}

	err := setup.cache.Set(setup.ctx, odds)
	require.NoError(t, err)

	// Check TTL is set
	key := "odds:event-123:match_winner:Team A"
	ttl := setup.miniRedis.TTL(key)
	assert.True(t, ttl > 0)
	assert.True(t, ttl <= 15*time.Minute)
}

// TestNewRedisCache_Configuration tests cache creation with different configurations
func TestNewRedisCache_Configuration(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	logger := zerolog.Nop()

	configs := []RedisCacheConfig{
		{
			Addr:     mr.Addr(),
			Password: "",
			DB:       0,
			TTL:      5 * time.Minute,
		},
		{
			Addr:     mr.Addr(),
			Password: "",
			DB:       1,
			TTL:      30 * time.Minute,
		},
		{
			Addr:     mr.Addr(),
			Password: "test-password",
			DB:       0,
			TTL:      1 * time.Hour,
		},
	}

	for _, config := range configs {
		cache := NewRedisCache(config, logger)
		assert.NotNil(t, cache)
		assert.Equal(t, config.TTL, cache.ttl)
		cache.Close()
	}
}
