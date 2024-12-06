package repository

import (
	"context"
	"yt-text/models"
)

type VideoRepository interface {
	Save(ctx context.Context, video *models.Video) error
	Find(ctx context.Context, id string) (*models.Video, error)
	FindByURL(ctx context.Context, url string) (*models.Video, error)
}
