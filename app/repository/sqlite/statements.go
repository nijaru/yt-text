package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/nijaru/yt-text/errors"
)

const (
	createVideoQuery = `
        INSERT INTO videos (
            id, url, status, transcription, summary,
            model_info, error, created_at, updated_at
        ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
    `

	getVideoQuery = `
        SELECT id, url, status, transcription, summary,
               model_info, error, created_at, updated_at
        FROM videos WHERE id = ?
    `

	getVideoByURLQuery = `
        SELECT id, url, status, transcription, summary,
               model_info, error, created_at, updated_at
        FROM videos WHERE url = ?
    `

	updateVideoQuery = `
        UPDATE videos SET
            status = ?,
            transcription = ?,
            summary = ?,
            model_info = ?,
            error = ?,
            updated_at = ?
        WHERE id = ?
    `

	deleteVideoQuery = `
        DELETE FROM videos WHERE id = ?
    `

	getStaleJobsQuery = `
        SELECT id, url, status, transcription, summary,
               model_info, error, created_at, updated_at
        FROM videos
        WHERE status = ? AND updated_at < ?
    `
)

type PreparedStatements struct {
	create   *sql.Stmt
	get      *sql.Stmt
	getByURL *sql.Stmt
	update   *sql.Stmt
	delete   *sql.Stmt
	getStale *sql.Stmt
}

func (stmts *PreparedStatements) Prepare(ctx context.Context, db *sql.DB) error {
	const op = "PreparedStatements.Prepare"

	var err error

	if stmts.create, err = db.PrepareContext(ctx, createVideoQuery); err != nil {
		return errors.Internal(op, err, "failed to prepare create statement")
	}

	if stmts.get, err = db.PrepareContext(ctx, getVideoQuery); err != nil {
		return errors.Internal(op, err, "failed to prepare get statement")
	}

	if stmts.getByURL, err = db.PrepareContext(ctx, getVideoByURLQuery); err != nil {
		return errors.Internal(op, err, "failed to prepare getByURL statement")
	}

	if stmts.update, err = db.PrepareContext(ctx, updateVideoQuery); err != nil {
		return errors.Internal(op, err, "failed to prepare update statement")
	}

	if stmts.delete, err = db.PrepareContext(ctx, deleteVideoQuery); err != nil {
		return errors.Internal(op, err, "failed to prepare delete statement")
	}

	if stmts.getStale, err = db.PrepareContext(ctx, getStaleJobsQuery); err != nil {
		return errors.Internal(op, err, "failed to prepare getStale statement")
	}

	return nil
}

func (stmts *PreparedStatements) Close() error {
	var errs []error

	statements := [...]*sql.Stmt{
		stmts.create,
		stmts.get,
		stmts.getByURL,
		stmts.update,
		stmts.delete,
		stmts.getStale,
	}

	for _, stmt := range statements {
		if stmt != nil {
			if err := stmt.Close(); err != nil {
				errs = append(errs, err)
			}
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("failed to close prepared statements: %v", errs)
	}

	return nil
}
