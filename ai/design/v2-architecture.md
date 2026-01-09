# yt-text v2 Architecture

**Status**: Implemented (local tested, cloud written)
**Date**: 2026-01-08
**Goal**: Production transcription service using Parakeet, optimized for Cloudflare free tier + Modal GPU

## Overview

Split architecture: Cloudflare (API/frontend, free) + Modal (GPU compute, pay-per-use)

```
┌─────────────────────────────────────────────────────────────────┐
│                    Cloudflare (Free Tier)                        │
│  ┌─────────────┐    ┌─────────────┐    ┌─────────────────────┐  │
│  │   Pages     │    │   Workers   │    │   D1 + R2 + Queues  │  │
│  │  (htmx UI)  │◀──▶│ (Hono API)  │◀──▶│     (Storage)       │  │
│  └─────────────┘    └──────┬──────┘    └─────────────────────┘  │
│                            │                                     │
│         SSE ◀──────────────┤                                     │
│     (real-time)            │ Queue                               │
└────────────────────────────┼────────────────────────────────────┘
                             ▼
                    ┌─────────────────┐
                    │   Modal (GPU)   │
                    │   Parakeet +    │
                    │     yt-dlp      │
                    │                 │
                    │  Webhook back   │──▶ Worker updates D1
                    └─────────────────┘
```

## Stack Decisions

| Layer    | Technology         | Rationale                       |
| -------- | ------------------ | ------------------------------- |
| Frontend | htmx + Tailwind    | Minimal JS, server-driven, fast |
| API      | Hono on CF Workers | TypeScript, edge-native, fast   |
| Queue    | Cloudflare Queues  | Native integration, reliable    |
| Database | Cloudflare D1      | SQLite at edge, 5GB free        |
| Storage  | Cloudflare R2      | S3-compatible, 10GB free        |
| Compute  | Modal (L4 GPU)     | Serverless GPU, Parakeet        |
| ASR      | Parakeet TDT 0.6B  | Best accuracy/speed ratio       |

## API Design

### Endpoints

```
POST /api/transcribe
  Body: { url: string, language?: string }
  Response: { jobId: string, status: "queued" }

GET /api/jobs/:id
  Response: { jobId, status, progress, result?, error? }

GET /api/jobs/:id/stream
  Response: SSE stream of status updates

GET /api/jobs/:id/result
  Response: { text, duration, wordCount, model }

POST /api/jobs/:id/retry
  Response: { jobId, status: "queued" }
```

### Job States

```
queued → downloading → transcribing → complete
                ↓              ↓
              failed ←───────┘
```

## Data Flow

1. **Submit**: User POSTs URL → Worker creates job in D1 → Enqueues to CF Queue
2. **Process**: Queue consumer triggers Modal webhook → Modal downloads + transcribes
3. **Callback**: Modal calls Worker webhook with result → Worker updates D1, stores in R2
4. **Poll/Stream**: Frontend uses SSE to get real-time updates from Worker

## Frontend Design

htmx with server-rendered partials:

```html
<!-- Main form -->
<form hx-post="/api/transcribe" hx-target="#result">
  <input name="url" placeholder="YouTube URL..." />
  <button>Transcribe</button>
</form>

<!-- Result container, updated via htmx -->
<div id="result"></div>
```

Server returns HTML fragments, not JSON. Simpler, faster, progressive enhancement.

## Modal Worker

```python
import modal

app = modal.App("yt-text")

image = (
    modal.Image.debian_slim(python_version="3.12")
    .apt_install("ffmpeg")
    .pip_install("yt-dlp", "nemo_toolkit[asr]", "httpx")
)

@app.function(
    image=image,
    gpu="L4",
    timeout=1800,
    secrets=[modal.Secret.from_name("cloudflare")],
)
async def transcribe(job_id: str, url: str, callback_url: str):
    import subprocess
    import nemo.collections.asr as nemo_asr
    import httpx

    # Download audio
    audio_path = f"/tmp/{job_id}.wav"
    subprocess.run([
        "yt-dlp", "-x", "--audio-format", "wav",
        "-o", audio_path, url
    ], check=True)

    # Transcribe with Parakeet
    model = nemo_asr.models.ASRModel.from_pretrained("nvidia/parakeet-tdt-0.6b-v2")
    result = model.transcribe([audio_path])

    # Callback to Cloudflare Worker
    async with httpx.AsyncClient() as client:
        await client.post(callback_url, json={
            "job_id": job_id,
            "status": "complete",
            "text": result[0].text,
        })
```

## Cost Analysis

### Cloudflare (Free Tier)

| Resource         | Limit    | Expected Usage     |
| ---------------- | -------- | ------------------ |
| Workers requests | 100K/day | Well under         |
| D1 storage       | 5GB      | ~1MB per 1000 jobs |
| D1 reads         | 5M/day   | Well under         |
| R2 storage       | 10GB     | Cache results      |
| Queues           | 1M/month | Well under         |

### Modal

| Resource       | Cost      | Usage                   |
| -------------- | --------- | ----------------------- |
| L4 GPU         | $0.80/hr  | ~1 min per 1-hour audio |
| Free credits   | $30/month | ~37 GPU hours           |
| Parakeet speed | 3000x RTF | Very efficient          |

**Effective cost**: ~$0.013 per hour of audio transcribed (after free credits)

## Implementation Phases

### Phase 1: Cleanup (Current)

- Remove tracked database files
- Consolidate/remove old docs
- Update .gitignore
- Remove duplicate static/ frontend

### Phase 2: Modal Worker

- Set up Modal project
- Implement Parakeet transcription
- Test with direct invocation
- Add webhook callback

### Phase 3: Cloudflare API

- Initialize Hono project
- Set up D1 schema
- Implement job endpoints
- Add Queue consumer
- Implement SSE streaming

### Phase 4: Frontend

- htmx + Tailwind setup
- Form and result components
- Progress display
- Error handling

### Phase 5: Integration

- Wire up Queue → Modal
- Wire up Modal → Webhook
- End-to-end testing
- Deploy to production

## Alternative Considered

**All-Python on Modal**: Simpler but worse UX (no edge API, slower responses).

**All-Cloudflare with external GPU API**: Could use Replicate instead of Modal, but less control.

**Current Python stack**: Good architecture but wrong compute target (VPS vs serverless GPU).

## Open Questions

1. **Auth**: Add Cloudflare Access? Simple API keys? None for public demo?
2. **Rate limits**: Per-IP? Per-API-key? How strict?
3. **Result storage**: Keep forever? TTL? User-deletable?
4. **Multilingual**: Parakeet v3 (25 langs) vs v2 (English only)?
