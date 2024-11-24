package repository

import (
	"context"

	"github.com/nijaru/yt-text/models"
)

type VideoRepository interface {
	// Core CRUD operations
	Create(ctx context.Context, video *models.Video) error
	Get(ctx context.Context, id string) (*models.Video, error)
	GetByURL(ctx context.Context, url string) (*models.Video, error)
	Update(ctx context.Context, video *models.Video) error
	Delete(ctx context.Context, id string) error

	// Transaction support
	WithTx(ctx context.Context, fn func(repo VideoRepository) error) error
}
