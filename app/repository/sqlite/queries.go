package sqlite

const (
	insertQuery = `
        INSERT INTO videos (
            id, url, title, status, transcription,
            error, created_at, updated_at
        ) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
        ON CONFLICT(id) DO UPDATE SET
            title = excluded.title,
            status = excluded.status,
            transcription = excluded.transcription,
            error = excluded.error,
            updated_at = excluded.updated_at
    `

	getQuery = `
        SELECT id, url, title, status, transcription,
               error, created_at, updated_at
        FROM videos WHERE id = ?
    `

	getByURLQuery = `
        SELECT id, url, title, status, transcription,
               error, created_at, updated_at
        FROM videos WHERE url = ?
    `

	updateQuery = `
        UPDATE videos SET
            title = ?,
            status = ?,
            transcription = ?,
            error = ?,
            updated_at = ?
        WHERE id = ?
    `
)
