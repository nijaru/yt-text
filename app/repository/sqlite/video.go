package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/nijaru/yt-text/errors"
	"github.com/nijaru/yt-text/models"
	"github.com/nijaru/yt-text/repository"
)

type SQLiteRepository struct {
	db        Executor
	config    DBConfig
	stmts     *PreparedStatements
	stmtMutex sync.RWMutex // Protects prepared statements
}

// NewRepository creates a new SQLite repository
func NewRepository(db *sql.DB) (repository.VideoRepository, error) {
	repo := &SQLiteRepository{
		db:     db,
		config: DefaultDBConfig(),
		stmts:  &PreparedStatements{},
	}

	// Prepare statements with timeout context
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := repo.stmts.Prepare(ctx, db); err != nil {
		return nil, fmt.Errorf("failed to prepare statements: %w", err)
	}

	return repo, nil
}

// Close closes the repository and its prepared statements
func (r *SQLiteRepository) Close() error {
	r.stmtMutex.Lock()
	defer r.stmtMutex.Unlock()

	if r.stmts != nil {
		return r.stmts.Close()
	}
	return nil
}

func (r *SQLiteRepository) WithTx(ctx context.Context, fn func(repository.VideoRepository) error) error {
	const op = "SQLiteRepository.WithTx"

	// Only start a new transaction if we're not already in one
	if _, ok := r.db.(*sql.Tx); ok {
		return fn(r)
	}

	db, ok := r.db.(*sql.DB)
	if !ok {
		return errors.Internal(op, nil, "invalid database connection")
	}

	return WithTransaction(ctx, db, func(tx Executor) error {
		txRepo := &SQLiteRepository{
			db:     tx,
			config: r.config,
		}
		return fn(txRepo)
	})
}

func (r *SQLiteRepository) Create(ctx context.Context, video *models.Video) error {
	const op = "SQLiteRepository.Create"

	r.stmtMutex.RLock()
	defer r.stmtMutex.RUnlock()

	// Marshal JSON fields
	modelInfo, err := json.Marshal(video.ModelInfo)
	if err != nil {
		return errors.Internal(op, err, "failed to marshal model info")
	}

	progress, err := json.Marshal(video.Progress)
	if err != nil {
		return errors.Internal(op, err, "failed to marshal progress")
	}

	result, err := r.stmts.create.ExecContext(ctx,
		video.ID,
		video.URL,
		string(video.Status),
		video.Transcription,
		video.Summary,
		modelInfo,
		video.Error,
		progress,
		video.CreatedAt,
		video.UpdatedAt,
	)
	if err != nil {
		return errors.Internal(op, err, "failed to create video record")
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return errors.Internal(op, err, "failed to get rows affected")
	}
	if rows != 1 {
		return errors.Internal(op, nil, "expected 1 row affected")
	}

	return nil
}

func (r *SQLiteRepository) Get(ctx context.Context, id string) (*models.Video, error) {
	const op = "SQLiteRepository.Get"

	r.stmtMutex.RLock()
	defer r.stmtMutex.RUnlock()

	video, err := r.scanVideo(r.stmts.get.QueryRowContext(ctx, id))
	if err != nil {
		return nil, errors.Internal(op, err, "failed to get video")
	}
	if video == nil {
		return nil, errors.NotFound(op, nil, "video not found")
	}

	return video, nil
}

func (r *SQLiteRepository) GetByURL(ctx context.Context, url string) (*models.Video, error) {
	const op = "SQLiteRepository.GetByURL"

	r.stmtMutex.RLock()
	defer r.stmtMutex.RUnlock()

	video, err := r.scanVideo(r.stmts.getByURL.QueryRowContext(ctx, url))
	if err != nil {
		return nil, errors.Internal(op, err, "failed to get video by URL")
	}

	return video, nil
}

func (r *SQLiteRepository) Update(ctx context.Context, video *models.Video) error {
	const op = "SQLiteRepository.Update"

	r.stmtMutex.RLock()
	defer r.stmtMutex.RUnlock()

	// Marshal JSON fields
	modelInfo, err := json.Marshal(video.ModelInfo)
	if err != nil {
		return errors.Internal(op, err, "failed to marshal model info")
	}

	progress, err := json.Marshal(video.Progress)
	if err != nil {
		return errors.Internal(op, err, "failed to marshal progress")
	}

	result, err := r.stmts.update.ExecContext(ctx,
		string(video.Status),
		video.Transcription,
		video.Summary,
		modelInfo,
		video.Error,
		progress,
		time.Now(), // updated_at
		video.ID,
	)
	if err != nil {
		return errors.Internal(op, err, "failed to update video")
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return errors.Internal(op, err, "failed to get rows affected")
	}
	if rows == 0 {
		return errors.NotFound(op, nil, "video not found")
	}

	return nil
}

func (r *SQLiteRepository) Delete(ctx context.Context, id string) error {
	const op = "SQLiteRepository.Delete"

	r.stmtMutex.RLock()
	defer r.stmtMutex.RUnlock()

	result, err := r.stmts.delete.ExecContext(ctx, id)
	if err != nil {
		return errors.Internal(op, err, "failed to delete video")
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return errors.Internal(op, err, "failed to get rows affected")
	}
	if rows == 0 {
		return errors.NotFound(op, nil, "video not found")
	}

	return nil
}

// Additional helper methods

func (r *SQLiteRepository) GetStaleJobs(ctx context.Context, timeout time.Duration) ([]*models.Video, error) {
	const op = "SQLiteRepository.GetStaleJobs"

	r.stmtMutex.RLock()
	defer r.stmtMutex.RUnlock()

	cutoff := time.Now().Add(-timeout)
	rows, err := r.stmts.getStale.QueryContext(ctx, string(models.StatusProcessing), cutoff)
	if err != nil {
		return nil, errors.Internal(op, err, "failed to query stale jobs")
	}
	defer rows.Close()

	videos, err := r.scanVideos(rows)
	if err != nil {
		return nil, errors.Internal(op, err, "failed to scan stale jobs")
	}

	return videos, nil
}

func (r *SQLiteRepository) scanVideo(row *sql.Row) (*models.Video, error) {
	const op = "SQLiteRepository.scanVideo"

	video := &models.Video{}
	var modelInfoJSON, progressJSON []byte
	var status string

	err := row.Scan(
		&video.ID,
		&video.URL,
		&status,
		&video.Transcription,
		&video.Summary,
		&modelInfoJSON,
		&video.Error,
		&progressJSON,
		&video.CreatedAt,
		&video.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, errors.Internal(op, err, "failed to scan video row")
	}

	video.Status = models.Status(status)

	if err := json.Unmarshal(modelInfoJSON, &video.ModelInfo); err != nil {
		return nil, errors.Internal(op, err, "failed to unmarshal model info")
	}
	if err := json.Unmarshal(progressJSON, &video.Progress); err != nil {
		return nil, errors.Internal(op, err, "failed to unmarshal progress")
	}

	return video, nil
}

func (r *SQLiteRepository) scanVideos(rows *sql.Rows) ([]*models.Video, error) {
	const op = "SQLiteRepository.scanVideos"

	var videos []*models.Video
	for rows.Next() {
		video := &models.Video{}
		var modelInfoJSON, progressJSON []byte
		var status string

		err := rows.Scan(
			&video.ID,
			&video.URL,
			&status,
			&video.Transcription,
			&video.Summary,
			&modelInfoJSON,
			&video.Error,
			&progressJSON,
			&video.CreatedAt,
			&video.UpdatedAt,
		)
		if err != nil {
			return nil, errors.Internal(op, err, "failed to scan video row")
		}

		video.Status = models.Status(status)

		if err := json.Unmarshal(modelInfoJSON, &video.ModelInfo); err != nil {
			return nil, errors.Internal(op, err, "failed to unmarshal model info")
		}
		if err := json.Unmarshal(progressJSON, &video.Progress); err != nil {
			return nil, errors.Internal(op, err, "failed to unmarshal progress")
		}

		videos = append(videos, video)
	}

	if err := rows.Err(); err != nil {
		return nil, errors.Internal(op, err, "error iterating video rows")
	}

	return videos, nil
}
