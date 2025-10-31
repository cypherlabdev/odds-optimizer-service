package service

import (
	"context"

	"github.com/cypherlabdev/odds-optimizer-service/internal/models"
)

// Cache is an interface that abstracts cache operations
// This allows for easier testing and mocking
type Cache interface {
	Set(ctx context.Context, odds *models.OptimizedOdds) error
	Get(ctx context.Context, eventID, market, selection string) (*models.OptimizedOdds, error)
	SetBatch(ctx context.Context, oddsList []*models.OptimizedOdds) error
	GetByEvent(ctx context.Context, eventID string) ([]*models.OptimizedOdds, error)
	Ping(ctx context.Context) error
	Close() error
}
