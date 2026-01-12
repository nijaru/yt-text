-- D1 Database Schema for yt-text
-- Run: wrangler d1 execute yt-text-db --file schema.sql

CREATE TABLE IF NOT EXISTS jobs (
    id TEXT PRIMARY KEY,
    url TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'queued',
    progress INTEGER NOT NULL DEFAULT 0,
    language TEXT DEFAULT 'en',
    mode TEXT DEFAULT 'transcribe',  -- 'transcribe' or 'extract'
    text TEXT,
    error TEXT,
    duration INTEGER,
    word_count INTEGER,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_jobs_status ON jobs(status);
CREATE INDEX IF NOT EXISTS idx_jobs_created_at ON jobs(created_at);
