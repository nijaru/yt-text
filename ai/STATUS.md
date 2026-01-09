# Project Status

**Updated**: 2026-01-08
**State**: Portfolio-ready, archived

## Current State

Complete rewrite from Python/Litestar to modern serverless architecture:

| Component          | Status  | Notes                       |
| ------------------ | ------- | --------------------------- |
| `local/cli.py`     | Tested  | MLX backend works on M3 Max |
| `modal/app.py`     | Written | NeMo/Parakeet, untested     |
| `api/src/index.ts` | Written | Hono + htmx, untested       |

## What Was Done

1. Removed old Python/Litestar codebase (~10,900 lines)
2. Removed Solid.js frontend
3. Removed Docker/deployment configs
4. Created new architecture:
   - Hono API for Cloudflare Workers
   - Modal worker for GPU transcription
   - Multi-backend local CLI (MLX/NeMo/ONNX)
5. Tested local CLI with parakeet-mlx

## Test Results

```
Input: "Me at the zoo" (19s YouTube video)
Output: 41 words, 23.2s transcription time
Backend: parakeet-mlx on M3 Max
```

## Architecture

Portfolio-optimized: working local demo + cloud-ready code (not deployed)

```
Local (tested)     Cloud (written, not deployed)
┌───────────┐      ┌──────────────────────────────┐
│ cli.py    │      │ CF Workers → Modal GPU       │
│ parakeet  │      │ Hono/htmx    Parakeet/NeMo   │
│ mlx       │      │ D1/R2/Queue  L4 GPU          │
└───────────┘      └──────────────────────────────┘
```

## Next Steps (if resumed)

- Test Modal worker with `modal run modal/app.py`
- Test Hono API with `bun run dev`
- Integration testing
- Or: archive as-is for portfolio
