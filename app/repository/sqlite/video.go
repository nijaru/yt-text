package sqlite

import (
	"context"
	"database/sql"
	"strings"
	"time"
	"yt-text/errors"
	"yt-text/models"
)

type Repository struct {
	db *DB
}

func NewRepository(db *DB) (*Repository, error) {
	return &Repository{db: db}, nil
}

func (r *Repository) Save(ctx context.Context, video *models.Video) error {
	const op = "SQLiteRepository.Save"

	for i := 0; i < 3; i++ { // Simple retry logic
		err := r.save(ctx, video)
		if err == nil {
			return nil
		}
		if !isLockError(err) {
			return errors.Internal(op, err, "Failed to save video")
		}
		time.Sleep(time.Second * time.Duration(i+1))
	}
	return errors.Internal(op, nil, "Failed after retries")
}

func (r *Repository) save(ctx context.Context, video *models.Video) error {
	_, err := r.db.statements.insert.ExecContext(ctx,
		video.ID,
		video.URL,
		video.Title,
		string(video.Status),
		video.Transcription,
		video.Error,
		video.CreatedAt,
		video.UpdatedAt,
	)
	return err
}

func (r *Repository) Find(ctx context.Context, id string) (*models.Video, error) {
	const op = "SQLiteRepository.Find"

	video := &models.Video{}
	var status string

	err := r.db.statements.get.QueryRowContext(ctx, id).Scan(
		&video.ID,
		&video.URL,
		&video.Title,
		&status,
		&video.Transcription,
		&video.Error,
		&video.CreatedAt,
		&video.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, errors.NotFound(op, nil, "Video not found")
	}
	if err != nil {
		return nil, errors.Internal(op, err, "Failed to query video")
	}

	video.Status = models.Status(status)
	return video, nil
}

func (r *Repository) FindByURL(ctx context.Context, url string) (*models.Video, error) {
	const op = "SQLiteRepository.FindByURL"

	video := &models.Video{}
	var status string

	err := r.db.statements.getByURL.QueryRowContext(ctx, url).Scan(
		&video.ID,
		&video.URL,
		&video.Title,
		&status,
		&video.Transcription,
		&video.Error,
		&video.CreatedAt,
		&video.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, errors.NotFound(op, nil, "Video not found")
	}
	if err != nil {
		return nil, errors.Internal(op, err, "Failed to query video")
	}

	video.Status = models.Status(status)
	return video, nil
}

func isLockError(err error) bool {
	return strings.Contains(err.Error(), "database is locked") ||
		strings.Contains(err.Error(), "busy")
}
