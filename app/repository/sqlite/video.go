package sqlite

import (
	"context"
	"database/sql"
	"yt-text/errors"
	"yt-text/models"
)

type SQLiteRepository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) (*SQLiteRepository, error) {
	return &SQLiteRepository{db: db}, nil
}

func (r *SQLiteRepository) Save(ctx context.Context, video *models.Video) error {
	const op = "SQLiteRepository.Save"

	query := `
        INSERT INTO videos (id, url, status, transcription, error, created_at, updated_at)
        VALUES (?, ?, ?, ?, ?, ?, ?)
        ON CONFLICT(id) DO UPDATE SET
            status = excluded.status,
            transcription = excluded.transcription,
            error = excluded.error,
            updated_at = excluded.updated_at
    `

	_, err := r.db.ExecContext(ctx, query,
		video.ID,
		video.URL,
		string(video.Status),
		video.Transcription,
		video.Error,
		video.CreatedAt,
		video.UpdatedAt,
	)
	if err != nil {
		return errors.Internal(op, err, "Failed to save video")
	}

	return nil
}

func (r *SQLiteRepository) Find(ctx context.Context, id string) (*models.Video, error) {
	const op = "SQLiteRepository.Find"

	query := `
        SELECT id, url, status, transcription, error, created_at, updated_at
        FROM videos WHERE id = ?
    `

	video := &models.Video{}
	var status string

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&video.ID,
		&video.URL,
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

func (r *SQLiteRepository) FindByURL(ctx context.Context, url string) (*models.Video, error) {
	const op = "SQLiteRepository.FindByURL"

	query := `
        SELECT id, url, status, transcription, error, created_at, updated_at
        FROM videos WHERE url = ?
    `

	video := &models.Video{}
	var status string

	err := r.db.QueryRowContext(ctx, query, url).Scan(
		&video.ID,
		&video.URL,
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
