package service

import (
	"github.com/cypherlabdev/odds-optimizer-service/internal/models"
)

// Optimizer is an interface that abstracts odds optimization operations
// This allows for easier testing and mocking
type Optimizer interface {
	Optimize(normalized *models.NormalizedOdds) (*models.OptimizedOdds, error)
	BatchOptimize(normalized []*models.NormalizedOdds) ([]*models.OptimizedOdds, error)
}
