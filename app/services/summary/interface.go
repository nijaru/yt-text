package summary

import (
	"context"

	"github.com/nijaru/yt-text/models"
)

type Service interface {
	CreateSummary(ctx context.Context, url string, opts Options) (*models.Video, error)
	GetSummary(ctx context.Context, url string) (*models.Video, error)
	GetStatus(ctx context.Context, id string) (*models.Video, error)
}

type Repository interface {
	GetByURL(ctx context.Context, url string) (*models.Video, error)
	Update(ctx context.Context, video *models.Video) error
}

type Config struct {
	ModelName string
	MaxLength int
	MinLength int
	BatchSize int
	ChunkSize int
}

type Options struct {
	ModelName string
	MaxLength int
	MinLength int
}
