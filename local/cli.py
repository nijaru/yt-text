#!/usr/bin/env python3
"""
yt-text Local CLI

Transcribe videos locally using Parakeet MLX on Apple Silicon.
For development and demonstration purposes.

Usage:
    uv run cli.py <url-or-file>
    uv run cli.py https://youtube.com/watch?v=...
    uv run cli.py ~/audio.mp3
"""

import subprocess
import sys
import tempfile
import time
from pathlib import Path

from rich.console import Console
from rich.panel import Panel
from rich.progress import Progress, SpinnerColumn, TextColumn

console = Console()


def download_audio(url: str, output_dir: Path) -> Path:
    """Download audio from URL using yt-dlp."""
    output_path = output_dir / "audio.wav"

    result = subprocess.run(
        [
            "yt-dlp",
            "-x",
            "--audio-format",
            "wav",
            "--audio-quality",
            "0",
            "--postprocessor-args",
            "ffmpeg:-ar 16000 -ac 1",
            "-o",
            str(output_path.with_suffix(".%(ext)s")),
            url,
        ],
        capture_output=True,
        text=True,
    )

    if result.returncode != 0:
        raise RuntimeError(f"yt-dlp failed: {result.stderr}")

    # Find the actual output file
    wav_files = list(output_dir.glob("*.wav"))
    if not wav_files:
        raise RuntimeError("No audio file produced")

    return wav_files[0]


def transcribe_file(
    audio_path: Path, model_name: str = "mlx-community/parakeet-tdt-0.6b-v3"
) -> dict:
    """Transcribe audio file using Parakeet MLX."""
    from parakeet_mlx import from_pretrained

    model = from_pretrained(model_name)
    result = model.transcribe(str(audio_path))

    return {
        "text": result.text,
        "sentences": [{"text": s.text, "start": s.start, "end": s.end} for s in result.sentences]
        if hasattr(result, "sentences")
        else [],
    }


def format_duration(seconds: float) -> str:
    """Format seconds as MM:SS."""
    mins = int(seconds // 60)
    secs = int(seconds % 60)
    return f"{mins}:{secs:02d}"


def main():
    if len(sys.argv) < 2:
        console.print("[bold]yt-text[/] - Local video transcription\n")
        console.print("Usage: uv run cli.py <url-or-file>")
        console.print("\nExamples:")
        console.print("  uv run cli.py https://youtube.com/watch?v=...")
        console.print("  uv run cli.py ~/Downloads/podcast.mp3")
        sys.exit(1)

    input_path = sys.argv[1]
    start_time = time.time()

    # Check if input is a file or URL
    path = Path(input_path).expanduser()
    is_file = path.exists() and path.is_file()

    with Progress(
        SpinnerColumn(),
        TextColumn("[progress.description]{task.description}"),
        console=console,
    ) as progress:
        if is_file:
            # Direct file transcription
            console.print(f"\n[dim]File:[/] {path.name}")
            audio_path = path

            # Convert to WAV if needed
            if path.suffix.lower() != ".wav":
                task = progress.add_task("Converting to WAV...", total=None)
                with tempfile.TemporaryDirectory() as tmpdir:
                    wav_path = Path(tmpdir) / "audio.wav"
                    subprocess.run(
                        ["ffmpeg", "-i", str(path), "-ar", "16000", "-ac", "1", str(wav_path)],
                        capture_output=True,
                        check=True,
                    )
                    progress.remove_task(task)

                    task = progress.add_task("Transcribing with Parakeet...", total=None)
                    result = transcribe_file(wav_path)
                    progress.remove_task(task)
            else:
                task = progress.add_task("Transcribing with Parakeet...", total=None)
                result = transcribe_file(audio_path)
                progress.remove_task(task)
        else:
            # URL - need to download first
            console.print(f"\n[dim]URL:[/] {input_path}")

            with tempfile.TemporaryDirectory() as tmpdir:
                tmpdir_path = Path(tmpdir)

                task = progress.add_task("Downloading audio...", total=None)
                audio_path = download_audio(input_path, tmpdir_path)
                progress.remove_task(task)

                task = progress.add_task("Transcribing with Parakeet...", total=None)
                result = transcribe_file(audio_path)
                progress.remove_task(task)

    elapsed = time.time() - start_time
    word_count = len(result["text"].split())

    # Display result
    console.print()
    console.print(
        Panel(
            result["text"],
            title="[bold green]Transcription[/]",
            border_style="green",
            padding=(1, 2),
        )
    )

    console.print(f"\n[dim]Words:[/] {word_count}")
    console.print(f"[dim]Time:[/] {elapsed:.1f}s")

    # Show timestamps if available
    if result.get("sentences"):
        console.print("\n[bold]Timestamps:[/]")
        for s in result["sentences"][:5]:  # Show first 5
            console.print(f"  [{format_duration(s['start'])}] {s['text'][:60]}...")
        if len(result["sentences"]) > 5:
            console.print(f"  [dim]... and {len(result['sentences']) - 5} more[/]")


if __name__ == "__main__":
    main()
