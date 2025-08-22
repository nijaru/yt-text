# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with this repository.

## Project: yt-text

A web application that downloads and transcribes videos from YouTube and other platforms using yt-dlp and Whisper.

## Documentation Map

**Start here for any task:**
1. Check `docs/STATUS.md` - Current state and active work
2. Read `docs/TECH_SPEC.md` - Architecture and technical decisions
3. Review `docs/TODO.md` - Planned tasks and priorities

**Key documentation:**
- `docs/api/` - API design and endpoints
- `docs/implementation/` - Code patterns and conventions
- `docs/deployment/` - Infrastructure and deployment guides

## Quick Context

**Current State**: Migrating from Go/Python hybrid to pure Python with Litestar  
**Stack**: Python 3.12, uv, Litestar, SQLite, whisper.cpp  
**Goal**: Accurate transcription service optimized for small VPS deployment

## Critical Commands

```bash
# Development
uv sync                  # Install dependencies
uv run dev              # Run development server
uv run test             # Run tests

# Docker
docker compose up       # Start with docker
make docker-build       # Build image
```

## Working on Tasks

1. **Always check STATUS.md first** - Avoid duplicate work
2. **Update TODO.md** when starting/completing tasks
3. **Follow patterns in** `docs/implementation/patterns.md`
4. **Test locally before committing** - Use `uv run test`

## Navigation Rules

- **Don't read all docs** - Follow the hierarchy
- **Start narrow** - Read only what's needed for the task
- **Check examples** - Implementation docs have code samples
- **Update STATUS.md** - After significant changes