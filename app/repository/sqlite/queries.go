package sqlite

const (
	insertQuery = `
        INSERT INTO videos (
            id, url, title, status, transcription, transcription_path,
            source, error, language, language_probability, model_name,
            created_at, updated_at, last_accessed
        ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
        ON CONFLICT(id) DO UPDATE SET
            title = excluded.title,
            status = excluded.status,
            transcription = excluded.transcription,
            transcription_path = excluded.transcription_path,
            source = excluded.source,
            error = excluded.error,
            language = excluded.language,
            language_probability = excluded.language_probability,
            model_name = excluded.model_name,
            updated_at = excluded.updated_at,
            last_accessed = excluded.last_accessed
    `

	getQuery = `
        SELECT id, url, title, status, transcription, transcription_path,
               source, error, language, language_probability, model_name,
               created_at, updated_at, last_accessed
        FROM videos WHERE id = ?
    `

	getByURLQuery = `
        SELECT id, url, title, status, transcription, transcription_path,
               source, error, language, language_probability, model_name,
               created_at, updated_at, last_accessed
        FROM videos WHERE url = ?
    `

	updateQuery = `
        UPDATE videos SET
            title = ?,
            status = ?,
            transcription = ?,
            transcription_path = ?,
            source = ?,
            error = ?,
            language = ?,
            language_probability = ?,
            model_name = ?,
            updated_at = ?,
            last_accessed = ?
        WHERE id = ?
    `
    
    updateLastAccessedQuery = `
        UPDATE videos SET
            last_accessed = ?
        WHERE id = ?
    `
    
    findExpiredVideosQuery = `
        SELECT id, transcription_path
        FROM videos
        WHERE last_accessed < ?
    `
)
