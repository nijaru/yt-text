# CLAUDE.md

## Project: yt-text

Video transcription service using NVIDIA Parakeet ASR. Portfolio project demonstrating serverless edge + GPU architecture.

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

## Commands

```bash
# Local CLI (M3 Max)
cd local && uv run cli.py <url-or-file>

# API development
cd api && bun install && bun run dev

# Modal deployment
cd modal && modal deploy app.py
```

## Key Files

| File               | Purpose                                     |
| ------------------ | ------------------------------------------- |
| `api/src/index.ts` | Hono routes, htmx templates, queue consumer |
| `api/schema.sql`   | D1 database schema                          |
| `modal/app.py`     | GPU transcription with Parakeet             |
| `local/cli.py`     | Multi-backend CLI (MLX/NeMo/ONNX)           |

## Stack

- **API**: Hono + TypeScript on Cloudflare Workers
- **Database**: Cloudflare D1 (SQLite)
- **Queue**: Cloudflare Queues
- **Compute**: Modal with NVIDIA L4 GPU
- **ASR**: Parakeet TDT 0.6B
- **Frontend**: htmx + Tailwind (server-rendered)

## Design Decisions

1. **Split architecture**: Edge API (fast, free) + GPU compute (pay-per-use)
2. **htmx over SPA**: Server-rendered for simplicity
3. **Parakeet over Whisper**: 16x faster, better accuracy
4. **Model caching**: `@modal.cls` with `@modal.enter()` for warm starts
