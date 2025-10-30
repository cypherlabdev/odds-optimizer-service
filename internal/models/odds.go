package models

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// NormalizedOdds represents odds after normalization (from data-normalizer)
type NormalizedOdds struct {
	ID           uuid.UUID       `json:"id"`
	EventID      string          `json:"event_id"`
	EventName    string          `json:"event_name"`
	Sport        string          `json:"sport"`
	Competition  string          `json:"competition"`
	Market       string          `json:"market"`
	Selection    string          `json:"selection"`
	BackPrice    decimal.Decimal `json:"back_price"`
	LayPrice     decimal.Decimal `json:"lay_price"`
	BackSize     decimal.Decimal `json:"back_size"`
	LaySize      decimal.Decimal `json:"lay_size"`
	Timestamp    time.Time       `json:"timestamp"`
	NormalizedAt time.Time       `json:"normalized_at"`
}

// OptimizedOdds represents odds after ML optimization
type OptimizedOdds struct {
	ID              uuid.UUID       `json:"id"`
	EventID         string          `json:"event_id"`
	EventName       string          `json:"event_name"`
	Sport           string          `json:"sport"`
	Competition     string          `json:"competition"`
	Market          string          `json:"market"`
	Selection       string          `json:"selection"`
	OptimizedBack   decimal.Decimal `json:"optimized_back"`   // Optimized back price
	OptimizedLay    decimal.Decimal `json:"optimized_lay"`    // Optimized lay price
	OriginalBack    decimal.Decimal `json:"original_back"`
	OriginalLay     decimal.Decimal `json:"original_lay"`
	BackSize        decimal.Decimal `json:"back_size"`
	LaySize         decimal.Decimal `json:"lay_size"`
	Margin          decimal.Decimal `json:"margin"`           // Our profit margin
	Confidence      float64         `json:"confidence"`       // Model confidence (0-1)
	Timestamp       time.Time       `json:"timestamp"`
	OptimizedAt     time.Time       `json:"optimized_at"`
}

// OptimizationParams holds parameters for odds optimization
type OptimizationParams struct {
	MinMargin       decimal.Decimal // Minimum profit margin (e.g., 0.02 = 2%)
	MaxMargin       decimal.Decimal // Maximum profit margin (e.g., 0.10 = 10%)
	MinSpread       decimal.Decimal // Minimum back-lay spread
	TargetConfidence float64        // Target confidence level (0-1)
}

// KafkaNormalizedOddsMessage represents the Kafka message from data-normalizer
type KafkaNormalizedOddsMessage struct {
	OddsData  []NormalizedOdds `json:"odds_data"`
	Timestamp time.Time        `json:"timestamp"`
	BatchID   string           `json:"batch_id"`
}
