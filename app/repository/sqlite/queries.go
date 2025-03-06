package sqlite

const (
	insertQuery = `
        INSERT INTO videos (
            id, url, title, status, transcription,
            error, language, language_probability, model_name,
            created_at, updated_at
        ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
        ON CONFLICT(id) DO UPDATE SET
            title = excluded.title,
            status = excluded.status,
            transcription = excluded.transcription,
            error = excluded.error,
            language = excluded.language,
            language_probability = excluded.language_probability,
            model_name = excluded.model_name,
            updated_at = excluded.updated_at
    `

	getQuery = `
        SELECT id, url, title, status, transcription,
               error, language, language_probability, model_name,
               created_at, updated_at
        FROM videos WHERE id = ?
    `

	getByURLQuery = `
        SELECT id, url, title, status, transcription,
               error, language, language_probability, model_name,
               created_at, updated_at
        FROM videos WHERE url = ?
    `

	updateQuery = `
        UPDATE videos SET
            title = ?,
            status = ?,
            transcription = ?,
            error = ?,
            language = ?,
            language_probability = ?,
            model_name = ?,
            updated_at = ?
        WHERE id = ?
    `
)
