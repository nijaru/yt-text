# CLAUDE.md

## Project: yt-text

Video transcription service using NVIDIA Parakeet ASR models. Portfolio project demonstrating modern cloud architecture.

## Architecture

```
Cloudflare (Edge)              Modal (GPU Compute)
┌─────────────────────┐        ┌─────────────────┐
│  Workers (Hono API) │──Queue─│  Parakeet TDT   │
│  D1 (SQLite)        │◀─────callback───────────│
│  R2 (Storage)       │        │  yt-dlp         │
│  htmx Frontend      │        └─────────────────┘
└─────────────────────┘
```

## Project Structure

```
yt-text/
├── api/              # Hono API on Cloudflare Workers
│   ├── src/
│   │   └── index.ts  # Routes, htmx templates, queue consumer
│   ├── schema.sql    # D1 database schema
│   ├── wrangler.toml # Cloudflare config
│   └── package.json
├── modal/            # GPU worker on Modal
│   ├── app.py        # Parakeet transcription
│   └── pyproject.toml
├── local/            # Local dev (Apple Silicon)
│   ├── cli.py        # CLI using parakeet-mlx
│   └── pyproject.toml
└── ai/               # Design docs
    └── design/
        └── v2-architecture.md
```

## Commands

```bash
# Local development (M3 Max)
cd local && uv run cli.py <url-or-file>

# API development
cd api && bun install && bun run dev

# Modal deployment (if deploying)
cd modal && modal deploy app.py
```

## Stack

- **API**: Hono + TypeScript on Cloudflare Workers
- **Database**: Cloudflare D1 (SQLite)
- **Storage**: Cloudflare R2
- **Queue**: Cloudflare Queues
- **Compute**: Modal with NVIDIA L4 GPU
- **ASR**: Parakeet TDT 0.6B (NeMo on Modal, MLX locally)
- **Frontend**: htmx + Tailwind (server-rendered)

## Design Decisions

1. **Split architecture**: Edge API (fast, free) + GPU compute (pay-per-use)
2. **htmx over SPA**: Server-rendered for simplicity, minimal JS
3. **Parakeet over Whisper**: 16x faster, better accuracy
4. **Local dev with MLX**: Works on Apple Silicon without cloud costs
