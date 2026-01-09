# yt-text

Fast video transcription using NVIDIA Parakeet ASR. A portfolio project demonstrating modern serverless architecture with edge computing and GPU inference.

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                    Cloudflare (Free Tier)                        │
│  ┌─────────────┐    ┌─────────────┐    ┌─────────────────────┐  │
│  │   Workers   │    │     D1      │    │         R2          │  │
│  │  (Hono API) │───▶│  (SQLite)   │    │     (Storage)       │  │
│  │   + htmx    │    └─────────────┘    └─────────────────────┘  │
│  └──────┬──────┘                                                 │
│         │ Queue                                                  │
└─────────┼───────────────────────────────────────────────────────┘
          │
          ▼
┌─────────────────────────────────────────────────────────────────┐
│                        Modal (GPU)                               │
│  ┌─────────────────────────────────────────────────────────────┐│
│  │  Parakeet TDT 0.6B  •  yt-dlp  •  NVIDIA L4                 ││
│  │  3000x realtime  •  6% WER  •  $0.80/hr                     ││
│  └─────────────────────────────────────────────────────────────┘│
└─────────────────────────────────────────────────────────────────┘
```

## Why This Architecture?

| Decision                   | Rationale                                           |
| -------------------------- | --------------------------------------------------- |
| **Parakeet over Whisper**  | 16x faster, 40% lower error rate                    |
| **Edge API + GPU compute** | Fast responses globally, pay only for transcription |
| **htmx over SPA**          | Server-rendered, minimal JS, simpler                |
| **Cloudflare free tier**   | 100K requests/day, 5GB D1, 10GB R2                  |
| **Modal serverless GPU**   | Scale to zero, $30/month free credits               |

## Project Structure

```
yt-text/
├── api/                    # Cloudflare Workers API
│   ├── src/index.ts        # Hono routes + htmx templates
│   ├── schema.sql          # D1 database schema
│   └── wrangler.toml       # Cloudflare configuration
├── modal/                  # GPU transcription worker
│   ├── app.py              # Parakeet + yt-dlp
│   └── pyproject.toml
├── local/                  # Local development (Apple Silicon)
│   ├── cli.py              # CLI using parakeet-mlx
│   └── pyproject.toml
└── ai/design/              # Architecture documentation
```

## Local Development

Run transcription locally on Apple Silicon (M1/M2/M3):

```bash
cd local
uv sync
uv run cli.py https://youtube.com/watch?v=...
```

Or transcribe a local file:

```bash
uv run cli.py ~/Downloads/podcast.mp3
```

## API Development

```bash
cd api
bun install
bun run dev
```

Then open http://localhost:8787

## Deployment (Optional)

### Modal Worker

```bash
cd modal
uv sync
modal deploy app.py
```

### Cloudflare API

```bash
cd api

# Create D1 database
wrangler d1 create yt-text-db
wrangler d1 execute yt-text-db --file schema.sql

# Create R2 bucket
wrangler r2 bucket create yt-text-storage

# Create queue
wrangler queues create transcription-jobs

# Deploy
wrangler deploy
```

## Stack

| Layer    | Technology        | Purpose                      |
| -------- | ----------------- | ---------------------------- |
| Frontend | htmx + Tailwind   | Server-rendered UI           |
| API      | Hono + TypeScript | Edge routing, job management |
| Database | Cloudflare D1     | Job persistence              |
| Storage  | Cloudflare R2     | Audio files, results         |
| Queue    | Cloudflare Queues | Async job processing         |
| Compute  | Modal + L4 GPU    | Parakeet inference           |
| Local    | parakeet-mlx      | Apple Silicon development    |

## Performance

| Metric              | Value                                  |
| ------------------- | -------------------------------------- |
| Transcription speed | 3000x realtime (Modal) / 100x (M3 Max) |
| Word error rate     | ~6% (Parakeet TDT 0.6B)                |
| API latency         | <50ms (Cloudflare edge)                |
| Cold start          | ~10s (Modal with GPU snapshotting)     |

## Cost Analysis

**For a portfolio/demo project:**

| Service            | Free Tier    | Covers        |
| ------------------ | ------------ | ------------- |
| Cloudflare Workers | 100K req/day | API           |
| Cloudflare D1      | 5GB          | Database      |
| Cloudflare R2      | 10GB         | Storage       |
| Modal              | $30/month    | ~37 GPU hours |

**Effective cost**: Free for light usage, ~$0.01 per audio hour at scale.

## License

AGPL-3.0 - See [LICENSE](LICENSE)

## Acknowledgements

- [NVIDIA Parakeet](https://huggingface.co/nvidia/parakeet-tdt-0.6b-v2) - State-of-the-art ASR
- [parakeet-mlx](https://github.com/senstella/parakeet-mlx) - Apple Silicon port
- [yt-dlp](https://github.com/yt-dlp/yt-dlp) - Video downloading
- [Hono](https://hono.dev) - Edge-first web framework
- [htmx](https://htmx.org) - HTML-driven interactivity
- [Modal](https://modal.com) - Serverless GPU compute
