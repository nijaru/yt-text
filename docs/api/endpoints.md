# API Endpoints

## Base URL
- Development: `http://localhost:8000`
- Production: Configure via `API_BASE_URL`

## Endpoints

### POST /api/transcribe
Submit a URL for transcription.

**Request:**
```json
{
  "url": "https://youtube.com/watch?v=...",
  "model": "base",  // optional, defaults to base
  "language": "en"   // optional, auto-detect if not specified
}
```

**Response:**
```json
{
  "job_id": "550e8400-e29b-41d4-a716-446655440000",
  "status": "pending",
  "created_at": "2025-08-22T10:00:00Z"
}
```

**Status Codes:**
- 201: Job created
- 400: Invalid URL or parameters
- 429: Rate limit exceeded
- 503: Service unavailable

### GET /api/jobs/{job_id}
Check job status.

**Response:**
```json
{
  "job_id": "550e8400-e29b-41d4-a716-446655440000",
  "status": "completed",  // pending, processing, completed, failed
  "progress": 75,         // percentage if processing
  "created_at": "2025-08-22T10:00:00Z",
  "completed_at": "2025-08-22T10:05:00Z",
  "processing_time_ms": 300000
}
```

### GET /api/jobs/{job_id}/result
Get transcription result (only if completed).

**Response:**
```json
{
  "job_id": "550e8400-e29b-41d4-a716-446655440000",
  "url": "https://youtube.com/watch?v=...",
  "title": "Video Title",
  "duration": 180,  // seconds
  "text": "Full transcription text...",
  "model_used": "whisper-base",
  "word_count": 500,
  "language": "en"
}
```

### WebSocket /ws/jobs/{job_id}
Real-time job updates.

**Messages:**
```json
// Server -> Client
{
  "type": "status",
  "status": "processing",
  "progress": 45
}

{
  "type": "complete",
  "text": "Transcription text..."
}

{
  "type": "error",
  "error": "Failed to download video"
}
```

## Rate Limiting

Default limits (configurable):
- 10 requests/minute per IP
- 100 requests/day for API fallback
- 3 concurrent jobs per IP

Headers returned:
- `X-RateLimit-Limit`
- `X-RateLimit-Remaining`
- `X-RateLimit-Reset`

## Error Format

All errors return:
```json
{
  "error": {
    "code": "RATE_LIMIT_EXCEEDED",
    "message": "Too many requests",
    "details": {}  // optional context
  }
}
```

Error codes:
- `INVALID_URL`
- `RATE_LIMIT_EXCEEDED`
- `JOB_NOT_FOUND`
- `TRANSCRIPTION_FAILED`
- `SERVICE_UNAVAILABLE`