/**
 * yt-text API
 *
 * Cloudflare Workers API for video transcription.
 * Uses D1 for job storage, R2 for results, and Modal for GPU compute.
 */

import { Hono } from "hono";
import { cors } from "hono/cors";
import { logger } from "hono/logger";
import { streamSSE } from "hono/streaming";

type Bindings = {
	DB: D1Database;
	STORAGE: R2Bucket;
	JOBS_QUEUE: Queue;
	MODAL_TOKEN_ID: string;
	MODAL_TOKEN_SECRET: string;
	CALLBACK_SECRET: string;
	ENVIRONMENT: string;
};

type JobStatus =
	| "queued"
	| "downloading"
	| "transcribing"
	| "complete"
	| "failed";

interface Job {
	id: string;
	url: string;
	language: string;
	status: JobStatus;
	progress: number;
	text: string | null;
	error: string | null;
	duration: number | null;
	word_count: number | null;
	created_at: string;
	updated_at: string;
}

const app = new Hono<{ Bindings: Bindings }>();

// Utility: Escape HTML to prevent XSS
function escapeHtml(s: string): string {
	return s
		.replace(/&/g, "&amp;")
		.replace(/</g, "&lt;")
		.replace(/>/g, "&gt;")
		.replace(/"/g, "&quot;")
		.replace(/'/g, "&#039;");
}

// Utility: Format duration as HH:MM:SS or MM:SS
function formatDuration(seconds: number): string {
	const h = Math.floor(seconds / 3600);
	const m = Math.floor((seconds % 3600) / 60);
	const s = Math.floor(seconds % 60);
	if (h > 0) {
		return `${h}:${m.toString().padStart(2, "0")}:${s.toString().padStart(2, "0")}`;
	}
	return `${m}:${s.toString().padStart(2, "0")}`;
}

// Utility: Validate URL format and allowed domains
function validateUrl(url: string): { valid: boolean; error?: string } {
	try {
		const parsed = new URL(url);
		const allowed = ["youtube.com", "youtu.be", "www.youtube.com"];
		if (
			!allowed.some(
				(d) => parsed.hostname === d || parsed.hostname.endsWith(`.${d}`),
			)
		) {
			return { valid: false, error: "Only YouTube URLs are supported" };
		}
		return { valid: true };
	} catch {
		return { valid: false, error: "Invalid URL format" };
	}
}

// Middleware
app.use("*", logger());
app.use("/api/*", cors());

// Health check
app.get("/health", (c) =>
	c.json({ status: "ok", timestamp: new Date().toISOString() }),
);

// Home page (htmx)
app.get("/", (c) => {
	return c.html(renderPage());
});

// Submit transcription job (accepts form data from htmx)
app.post("/api/transcribe", async (c) => {
	let url: string;
	let language: string = "en";

	// Parse form data or JSON
	const contentType = c.req.header("Content-Type") || "";
	try {
		if (contentType.includes("application/x-www-form-urlencoded")) {
			const formData = await c.req.formData();
			url = formData.get("url") as string;
			language = (formData.get("language") as string) || "en";
		} else {
			const body = await c.req.json<{ url: string; language?: string }>();
			url = body.url;
			language = body.language || "en";
		}
	} catch {
		return c.json({ error: "Invalid request body" }, 400);
	}

	if (!url) {
		return c.json({ error: "URL is required" }, 400);
	}

	// Validate URL
	const validation = validateUrl(url);
	if (!validation.valid) {
		return c.json({ error: validation.error }, 400);
	}

	// Validate language (simple allowlist)
	const validLanguages = ["en", "es", "fr", "de", "it", "pt", "zh", "ja", "ko"];
	if (!validLanguages.includes(language)) {
		language = "en";
	}

	const id = crypto.randomUUID();
	const now = new Date().toISOString();

	// Insert job into D1
	await c.env.DB.prepare(
		`INSERT INTO jobs (id, url, status, progress, language, created_at, updated_at)
     VALUES (?, ?, 'queued', 0, ?, ?, ?)`,
	)
		.bind(id, url, language, now, now)
		.run();

	// Enqueue for processing
	await c.env.JOBS_QUEUE.send({
		jobId: id,
		url,
		language,
		callbackUrl: `${new URL(c.req.url).origin}/api/jobs/${id}/callback`,
	});

	// Return HTML partial for htmx or JSON for API clients
	if (contentType.includes("application/x-www-form-urlencoded")) {
		return c.html(
			renderProgress({
				id,
				url,
				language,
				status: "queued",
				progress: 0,
				text: null,
				error: null,
				duration: null,
				word_count: null,
				created_at: now,
				updated_at: now,
			}),
		);
	}

	return c.json({ jobId: id, status: "queued" });
});

// Get job status
app.get("/api/jobs/:id", async (c) => {
	const id = c.req.param("id");

	const result = await c.env.DB.prepare("SELECT * FROM jobs WHERE id = ?")
		.bind(id)
		.first<Job>();

	if (!result) {
		return c.json({ error: "Job not found" }, 404);
	}

	return c.json(result);
});

// Get job result (returns just the text for htmx)
app.get("/api/jobs/:id/result", async (c) => {
	const id = c.req.param("id");

	const job = await c.env.DB.prepare("SELECT * FROM jobs WHERE id = ?")
		.bind(id)
		.first<Job>();

	if (!job) {
		return c.json({ error: "Job not found" }, 404);
	}

	if (job.status === "complete" && job.text) {
		return c.json({
			text: job.text,
			duration: job.duration,
			wordCount: job.word_count,
		});
	}

	return c.json({ error: "Result not ready", status: job.status }, 202);
});

// SSE stream for job updates
app.get("/api/jobs/:id/stream", async (c) => {
	const id = c.req.param("id");

	return streamSSE(c, async (stream) => {
		let lastStatus = "";
		let lastProgress = -1;

		// Poll for updates (in production, use Durable Objects for real-time)
		for (let i = 0; i < 120; i++) {
			const job = await c.env.DB.prepare("SELECT * FROM jobs WHERE id = ?")
				.bind(id)
				.first<Job>();

			if (!job) {
				await stream.writeSSE({ event: "error", data: "Job not found" });
				break;
			}

			// Send update if status or progress changed
			if (job.status !== lastStatus || job.progress !== lastProgress) {
				lastStatus = job.status;
				lastProgress = job.progress;
				await stream.writeSSE({
					event: "status",
					data: JSON.stringify({
						status: job.status,
						progress: job.progress,
					}),
				});
			}

			if (job.status === "complete" || job.status === "failed") {
				await stream.writeSSE({
					event: "complete",
					data: JSON.stringify(job),
				});
				break;
			}

			await stream.sleep(1000);
		}
	});
});

// Callback from Modal worker (authenticated)
app.post("/api/jobs/:id/callback", async (c) => {
	// Verify callback secret
	const authHeader = c.req.header("Authorization");
	if (authHeader !== `Bearer ${c.env.CALLBACK_SECRET}`) {
		return c.json({ error: "Unauthorized" }, 401);
	}

	const id = c.req.param("id");
	let body: {
		status?: string;
		progress?: number;
		text?: string;
		duration?: number;
		word_count?: number;
		error?: string;
	};

	try {
		body = await c.req.json();
	} catch {
		return c.json({ error: "Invalid JSON body" }, 400);
	}

	const now = new Date().toISOString();

	if (body.status === "complete" && body.text) {
		await c.env.DB.prepare(
			`UPDATE jobs SET status = 'complete', text = ?, duration = ?, word_count = ?, updated_at = ?
       WHERE id = ?`,
		)
			.bind(body.text, body.duration ?? null, body.word_count ?? null, now, id)
			.run();
	} else if (body.error) {
		await c.env.DB.prepare(
			`UPDATE jobs SET status = 'failed', error = ?, updated_at = ? WHERE id = ?`,
		)
			.bind(body.error, now, id)
			.run();
	} else if (body.status && body.status !== "complete") {
		// Handle intermediate status updates (downloading, transcribing)
		await c.env.DB.prepare(
			`UPDATE jobs SET status = ?, progress = ?, updated_at = ? WHERE id = ?`,
		)
			.bind(body.status, body.progress ?? 0, now, id)
			.run();
	}

	return c.json({ ok: true });
});

// Retry failed job
app.post("/api/jobs/:id/retry", async (c) => {
	const id = c.req.param("id");

	const job = await c.env.DB.prepare("SELECT * FROM jobs WHERE id = ?")
		.bind(id)
		.first<Job>();

	if (!job) {
		return c.json({ error: "Job not found" }, 404);
	}

	if (job.status !== "failed") {
		return c.json({ error: "Can only retry failed jobs" }, 400);
	}

	const now = new Date().toISOString();
	await c.env.DB.prepare(
		`UPDATE jobs SET status = 'queued', error = NULL, progress = 0, updated_at = ? WHERE id = ?`,
	)
		.bind(now, id)
		.run();

	await c.env.JOBS_QUEUE.send({
		jobId: id,
		url: job.url,
		language: job.language || "en",
		callbackUrl: `${new URL(c.req.url).origin}/api/jobs/${id}/callback`,
	});

	// Return HTML partial for htmx
	const contentType = c.req.header("Content-Type") || "";
	if (
		contentType.includes("application/x-www-form-urlencoded") ||
		c.req.header("HX-Request")
	) {
		return c.html(
			renderProgress({
				...job,
				status: "queued",
				progress: 0,
				error: null,
				updated_at: now,
			}),
		);
	}

	return c.json({ jobId: id, status: "queued" });
});

// htmx partials
app.get("/partials/result/:id", async (c) => {
	const id = c.req.param("id");
	const job = await c.env.DB.prepare("SELECT * FROM jobs WHERE id = ?")
		.bind(id)
		.first<Job>();

	if (!job) {
		return c.html('<div class="text-red-500">Job not found</div>');
	}

	if (job.status === "complete" && job.text) {
		return c.html(renderResult(job));
	}

	if (job.status === "failed") {
		return c.html(renderError(job.error ?? "Unknown error", id));
	}

	// Still processing - return progress with polling
	return c.html(renderProgress(job));
});

// Queue consumer (processes jobs from Cloudflare Queue)
export default {
	fetch: app.fetch,

	async queue(
		batch: MessageBatch<{
			jobId: string;
			url: string;
			language?: string;
			callbackUrl: string;
		}>,
		env: Bindings,
	) {
		for (const message of batch.messages) {
			const { jobId, url, language, callbackUrl } = message.body;

			try {
				// Update status to downloading
				await env.DB.prepare(
					`UPDATE jobs SET status = 'downloading', updated_at = ? WHERE id = ?`,
				)
					.bind(new Date().toISOString(), jobId)
					.run();

				// Call Modal function
				const modalResponse = await fetch(
					"https://nijaru--yt-text-transcribe.modal.run",
					{
						method: "POST",
						headers: {
							"Content-Type": "application/json",
							Authorization: `Bearer ${env.MODAL_TOKEN_ID}:${env.MODAL_TOKEN_SECRET}`,
						},
						body: JSON.stringify({
							url,
							job_id: jobId,
							callback_url: callbackUrl,
							callback_secret: env.CALLBACK_SECRET,
							language: language ?? "en",
						}),
					},
				);

				if (!modalResponse.ok) {
					throw new Error(`Modal returned ${modalResponse.status}`);
				}

				message.ack();
			} catch (error) {
				// Sanitize URL before logging (remove potential tokens)
				const sanitizedUrl = url.split("?")[0];
				console.error(`Job ${jobId} failed for ${sanitizedUrl}:`, error);

				// Update job as failed
				await env.DB.prepare(
					`UPDATE jobs SET status = 'failed', error = ?, updated_at = ? WHERE id = ?`,
				)
					.bind(String(error), new Date().toISOString(), jobId)
					.run();

				message.ack(); // Don't retry on Modal errors
			}
		}
	},
};

// HTML Templates
function renderPage() {
	return `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>yt-text - Video Transcription</title>
  <script src="https://unpkg.com/htmx.org@2.0.4" integrity="sha384-HGfztofotfshcF7+8n44JQL2oJmowVChPTg48S+jvZoztPfvwD79OC/LTtG6dMp+" crossorigin="anonymous"></script>
  <script src="https://unpkg.com/htmx-ext-sse@2.2.2/sse.js" integrity="sha384-fw+eTlCc7suMV9Tl9wMpZhYjMD8gSjfDsUjlNnuLPNDCgHTv2FmNhpOsfstgbZCN" crossorigin="anonymous"></script>
  <script src="https://cdn.tailwindcss.com"></script>
  <link rel="icon" href="data:image/svg+xml,<svg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 100 100'><text y='.9em' font-size='90'>📝</text></svg>">
</head>
<body class="bg-zinc-950 text-zinc-100 min-h-screen">
  <div class="max-w-3xl mx-auto px-4 py-16">
    <header class="text-center mb-12">
      <h1 class="text-4xl font-bold mb-2">yt-text</h1>
      <p class="text-zinc-400">Fast video transcription powered by Parakeet</p>
    </header>

    <form
      hx-post="/api/transcribe"
      hx-target="#result"
      hx-swap="innerHTML"
      hx-indicator="#spinner"
      class="space-y-4"
    >
      <div class="flex gap-2">
        <input
          type="url"
          name="url"
          placeholder="Paste a YouTube URL..."
          required
          class="flex-1 bg-zinc-900 border border-zinc-800 rounded-lg px-4 py-3 text-lg focus:outline-none focus:border-zinc-600 transition-colors"
        >
        <button
          type="submit"
          class="bg-zinc-100 text-zinc-900 px-6 py-3 rounded-lg font-medium hover:bg-white transition-colors"
        >
          Transcribe
        </button>
      </div>
    </form>

    <div id="spinner" class="htmx-indicator text-center py-8">
      <div class="inline-block animate-spin rounded-full h-8 w-8 border-2 border-zinc-400 border-t-transparent"></div>
      <p class="text-zinc-400 mt-2">Submitting...</p>
    </div>

    <div id="result" class="mt-8"></div>
  </div>
</body>
</html>`;
}

function renderProgress(job: Job) {
	const statusText: Record<JobStatus, string> = {
		queued: "Waiting in queue...",
		downloading: "Downloading audio...",
		transcribing: "Transcribing with Parakeet...",
		complete: "Complete",
		failed: "Failed",
	};

	const status = job.status in statusText ? job.status : "queued";

	return `
<div
  hx-get="/partials/result/${escapeHtml(job.id)}"
  hx-trigger="every 2s"
  hx-swap="outerHTML"
  class="bg-zinc-900 border border-zinc-800 rounded-lg p-6"
>
  <div class="flex items-center gap-3">
    <div class="animate-spin rounded-full h-5 w-5 border-2 border-zinc-400 border-t-transparent"></div>
    <span class="text-zinc-300">${escapeHtml(statusText[status])}</span>
  </div>
  <div class="mt-4 h-2 bg-zinc-800 rounded-full overflow-hidden">
    <div class="h-full bg-zinc-400 transition-all duration-500" style="width: ${Math.min(100, Math.max(0, job.progress))}%"></div>
  </div>
</div>`;
}

function renderResult(job: Job) {
	const escapedText = escapeHtml(job.text ?? "");
	const escapedId = escapeHtml(job.id);

	return `
<div class="bg-zinc-900 border border-zinc-800 rounded-lg p-6 space-y-4">
  <div class="flex items-center justify-between">
    <div class="flex items-center gap-3 text-green-400">
      <svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" />
      </svg>
      <span>Transcription complete</span>
    </div>
    <div class="text-sm text-zinc-500">
      ${job.duration !== null ? formatDuration(job.duration) : ""}
      ${job.word_count ? `• ${job.word_count} words` : ""}
    </div>
  </div>

  <div class="bg-zinc-950 rounded-lg p-4 max-h-96 overflow-y-auto">
    <p class="text-zinc-200 whitespace-pre-wrap leading-relaxed" id="transcript-text">${escapedText}</p>
  </div>

  <div class="flex gap-2">
    <button
      onclick="navigator.clipboard.writeText(document.getElementById('transcript-text').textContent)"
      class="px-4 py-2 bg-zinc-800 hover:bg-zinc-700 rounded-lg text-sm transition-colors"
    >
      Copy
    </button>
    <button
      data-job-id="${escapedId}"
      onclick="downloadText(this.dataset.jobId)"
      class="px-4 py-2 bg-zinc-800 hover:bg-zinc-700 rounded-lg text-sm transition-colors"
    >
      Download
    </button>
  </div>
</div>

<script>
function downloadText(id) {
  const text = document.getElementById('transcript-text').textContent;
  const blob = new Blob([text], { type: 'text/plain' });
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = id + '.txt';
  a.click();
  URL.revokeObjectURL(url);
}
</script>`;
}

function renderError(error: string, jobId: string) {
	return `
<div class="bg-zinc-900 border border-red-900/50 rounded-lg p-6">
  <div class="flex items-center gap-3 text-red-400 mb-4">
    <svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
    </svg>
    <span>Transcription failed</span>
  </div>
  <p class="text-zinc-400 text-sm mb-4">${escapeHtml(error)}</p>
  <button
    hx-post="/api/jobs/${escapeHtml(jobId)}/retry"
    hx-target="#result"
    hx-swap="innerHTML"
    class="px-4 py-2 bg-zinc-800 hover:bg-zinc-700 rounded-lg text-sm transition-colors"
  >
    Retry
  </button>
</div>`;
}
