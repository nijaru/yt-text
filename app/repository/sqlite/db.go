package sqlite

import (
	"database/sql"

	_ "github.com/mattn/go-sqlite3"
)

type DB struct {
	*sql.DB
	statements *statements
}

type statements struct {
	insert   *sql.Stmt
	get      *sql.Stmt
	getByURL *sql.Stmt
	update   *sql.Stmt
}

func NewDB(path string) (*DB, error) {
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, err
	}

	// Configure database
	if err := setupDB(db); err != nil {
		db.Close()
		return nil, err
	}

	// Prepare statements
	stmts, err := prepareStatements(db)
	if err != nil {
		db.Close()
		return nil, err
	}

	return &DB{
		DB:         db,
		statements: stmts,
	}, nil
}

func setupDB(db *sql.DB) error {
	// Set pragmas for better performance
	pragmas := []string{
		"PRAGMA journal_mode = WAL",
		"PRAGMA synchronous = NORMAL",
		"PRAGMA foreign_keys = ON",
		"PRAGMA temp_store = MEMORY",
	}

	for _, pragma := range pragmas {
		if _, err := db.Exec(pragma); err != nil {
			return err
		}
	}

	// Create tables
	if err := createTables(db); err != nil {
		return err
	}

	return nil
}

func createTables(db *sql.DB) error {
	_, err := db.Exec(`
        CREATE TABLE IF NOT EXISTS videos (
            id TEXT PRIMARY KEY,
            url TEXT UNIQUE NOT NULL,
            title TEXT,
            status TEXT NOT NULL,
            transcription TEXT,
            error TEXT,
            created_at DATETIME NOT NULL,
            updated_at DATETIME NOT NULL
        );
        CREATE INDEX IF NOT EXISTS idx_videos_url ON videos(url);
        CREATE INDEX IF NOT EXISTS idx_videos_status ON videos(status);
    `)
	return err
}

func prepareStatements(db *sql.DB) (*statements, error) {
	// Prepare all statements
	insert, err := db.Prepare(insertQuery)
	if err != nil {
		return nil, err
	}

	get, err := db.Prepare(getQuery)
	if err != nil {
		insert.Close()
		return nil, err
	}

	getByURL, err := db.Prepare(getByURLQuery)
	if err != nil {
		insert.Close()
		get.Close()
		return nil, err
	}

	update, err := db.Prepare(updateQuery)
	if err != nil {
		insert.Close()
		get.Close()
		getByURL.Close()
		return nil, err
	}

	return &statements{
		insert:   insert,
		get:      get,
		getByURL: getByURL,
		update:   update,
	}, nil
}

func (db *DB) Close() error {
	if db.statements != nil {
		db.statements.insert.Close()
		db.statements.get.Close()
		db.statements.getByURL.Close()
		db.statements.update.Close()
	}
	return db.DB.Close()
}
