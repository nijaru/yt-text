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
	// Ensure LastAccessed is set if not already
	if video.LastAccessed.IsZero() {
		video.LastAccessed = video.UpdatedAt
	}
	
	_, err := r.db.statements.insert.ExecContext(ctx,
		video.ID,
		video.URL,
		video.Title,
		string(video.Status),
		video.Transcription,
		video.TranscriptionPath,
		string(video.Source),
		video.Error,
		video.Language,
		video.LanguageProbability,
		video.ModelName,
		video.CreatedAt,
		video.UpdatedAt,
		video.LastAccessed,
	)
	return err
}

func (r *Repository) Find(ctx context.Context, id string) (*models.Video, error) {
	const op = "SQLiteRepository.Find"

	video := &models.Video{}
	var status string
	var source sql.NullString

	err := r.db.statements.get.QueryRowContext(ctx, id).Scan(
		&video.ID,
		&video.URL,
		&video.Title,
		&status,
		&video.Transcription,
		&video.TranscriptionPath,
		&source,
		&video.Error,
		&video.Language,
		&video.LanguageProbability,
		&video.ModelName,
		&video.CreatedAt,
		&video.UpdatedAt,
		&video.LastAccessed,
	)

	if err == sql.ErrNoRows {
		return nil, errors.NotFound(op, nil, "Video not found")
	}
	if err != nil {
		return nil, errors.Internal(op, err, "Failed to query video")
	}

	video.Status = models.Status(status)
	
	// Set source if available, default to Whisper for backward compatibility
	if source.Valid {
		video.Source = models.TranscriptionSource(source.String)
	} else {
		video.Source = models.SourceWhisper
	}
	
	// Update last accessed time
	now := time.Now()
	_, err = r.db.statements.updateLastAccessed.ExecContext(ctx, now, id)
	if err != nil {
		// Log but don't fail the request
		// logger.Error("Failed to update last accessed time", "error", err, "id", id)
	} else {
		video.LastAccessed = now
	}
	
	return video, nil
}

func (r *Repository) FindByURL(ctx context.Context, url string) (*models.Video, error) {
	const op = "SQLiteRepository.FindByURL"

	video := &models.Video{}
	var status string
	var source sql.NullString

	err := r.db.statements.getByURL.QueryRowContext(ctx, url).Scan(
		&video.ID,
		&video.URL,
		&video.Title,
		&status,
		&video.Transcription,
		&video.TranscriptionPath,
		&source,
		&video.Error,
		&video.Language,
		&video.LanguageProbability,
		&video.ModelName,
		&video.CreatedAt,
		&video.UpdatedAt,
		&video.LastAccessed,
	)

	if err == sql.ErrNoRows {
		return nil, errors.NotFound(op, nil, "Video not found")
	}
	if err != nil {
		return nil, errors.Internal(op, err, "Failed to query video")
	}

	video.Status = models.Status(status)
	
	// Set source if available, default to Whisper for backward compatibility
	if source.Valid {
		video.Source = models.TranscriptionSource(source.String)
	} else {
		video.Source = models.SourceWhisper
	}
	
	// Update last accessed time
	now := time.Now()
	_, err = r.db.statements.updateLastAccessed.ExecContext(ctx, now, video.ID)
	if err != nil {
		// Log but don't fail the request
		// logger.Error("Failed to update last accessed time", "error", err, "id", video.ID)
	} else {
		video.LastAccessed = now
	}
	
	return video, nil
}

func isLockError(err error) bool {
	return strings.Contains(err.Error(), "database is locked") ||
		strings.Contains(err.Error(), "busy")
}

// FindExpiredVideos returns videos that haven't been accessed for a given duration
func (r *Repository) FindExpiredVideos(ctx context.Context, cutoff time.Time) ([]*models.ExpiredVideo, error) {
	const op = "SQLiteRepository.FindExpiredVideos"
	
	rows, err := r.db.statements.findExpiredVideos.QueryContext(ctx, cutoff)
	if err != nil {
		return nil, errors.Internal(op, err, "Failed to query expired videos")
	}
	defer rows.Close()
	
	var results []*models.ExpiredVideo
	for rows.Next() {
		var video models.ExpiredVideo
		if err := rows.Scan(&video.ID, &video.TranscriptionPath); err != nil {
			return nil, errors.Internal(op, err, "Failed to scan expired video")
		}
		results = append(results, &video)
	}
	
	return results, nil
}

// DeleteExpiredVideo deletes a video by ID
func (r *Repository) DeleteExpiredVideo(ctx context.Context, id string) error {
	const op = "SQLiteRepository.DeleteExpiredVideo"
	
	_, err := r.db.ExecContext(ctx, "DELETE FROM videos WHERE id = ?", id)
	if err != nil {
		return errors.Internal(op, err, "Failed to delete expired video")
	}
	
	return nil
}
