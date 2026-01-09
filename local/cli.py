#!/usr/bin/env python3
"""
yt-text Local CLI

Transcribe videos locally using Parakeet.
Automatically selects the best available backend:
- Apple Silicon: parakeet-mlx
- NVIDIA GPU: nemo_toolkit
- CPU: onnx-asr (slower)

Usage:
    uv run cli.py <url-or-file>
    uv run cli.py https://youtube.com/watch?v=...
    uv run cli.py ~/audio.mp3
    uv run cli.py audio.wav --backend mlx
"""

import platform
import subprocess
import sys
import tempfile
import time
from pathlib import Path

from rich.console import Console
from rich.panel import Panel
from rich.progress import Progress, SpinnerColumn, TextColumn

console = Console()


def detect_backend() -> str:
    """Detect the best available backend for this system."""
    system = platform.system()
    machine = platform.machine()

    # Apple Silicon
    if system == "Darwin" and machine == "arm64":
        try:
            import parakeet_mlx  # noqa: F401

            return "mlx"
        except ImportError:
            pass

    # NVIDIA GPU
    try:
        import torch

        if torch.cuda.is_available():
            try:
                import nemo.collections.asr  # noqa: F401

                return "nemo"
            except ImportError:
                pass
    except ImportError:
        pass

    # CPU fallback via ONNX
    try:
        import onnx_asr  # noqa: F401

        return "onnx"
    except ImportError:
        pass

    # Check what's actually installed and give helpful error
    console.print("[red]No transcription backend available.[/]\n")
    console.print("Install one of:")
    if system == "Darwin" and machine == "arm64":
        console.print("  [cyan]uv add parakeet-mlx[/]  (recommended for Apple Silicon)")
    console.print("  [cyan]uv add nemo_toolkit[asr][/]  (for NVIDIA GPU)")
    console.print("  [cyan]uv add onnx-asr[cpu,hub][/]  (CPU, slower)")
    sys.exit(1)


def download_audio(url: str, output_dir: Path) -> Path:
    """Download audio from URL using yt-dlp."""
    output_template = str(output_dir / "audio.%(ext)s")

    try:
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
                output_template,
                url,
            ],
            capture_output=True,
            text=True,
            timeout=300,  # 5 min download timeout
        )
    except subprocess.TimeoutExpired as e:
        raise RuntimeError("Download timed out after 5 minutes") from e

    if result.returncode != 0:
        raise RuntimeError(f"yt-dlp failed: {result.stderr}")

    # Find the output file
    wav_files = list(output_dir.glob("*.wav"))
    if not wav_files:
        raise RuntimeError("No audio file produced")

    return wav_files[0]


def convert_to_wav(input_path: Path, output_path: Path) -> None:
    """Convert audio to 16kHz mono WAV using ffmpeg."""
    try:
        result = subprocess.run(
            [
                "ffmpeg",
                "-y",
                "-i",
                str(input_path),
                "-ar",
                "16000",
                "-ac",
                "1",
                str(output_path),
            ],
            capture_output=True,
            text=True,
            timeout=120,  # 2 min conversion timeout
        )
        if result.returncode != 0:
            raise RuntimeError(f"ffmpeg failed: {result.stderr}")
    except subprocess.TimeoutExpired as e:
        raise RuntimeError("Audio conversion timed out") from e


def transcribe_mlx(audio_path: Path) -> dict:
    """Transcribe using parakeet-mlx (Apple Silicon)."""
    from parakeet_mlx import from_pretrained

    model = from_pretrained("mlx-community/parakeet-tdt-0.6b-v3")
    result = model.transcribe(str(audio_path))

    sentences = []
    if hasattr(result, "sentences"):
        sentences = [{"text": s.text, "start": s.start, "end": s.end} for s in result.sentences]

    return {"text": result.text, "sentences": sentences, "backend": "mlx"}


def transcribe_nemo(audio_path: Path) -> dict:
    """Transcribe using NeMo (NVIDIA GPU)."""
    import nemo.collections.asr as nemo_asr

    model = nemo_asr.models.ASRModel.from_pretrained("nvidia/parakeet-tdt-0.6b-v2")
    result = model.transcribe([str(audio_path)])

    text = result[0].text if hasattr(result[0], "text") else str(result[0])

    return {"text": text, "sentences": [], "backend": "nemo"}


def transcribe_onnx(audio_path: Path) -> dict:
    """Transcribe using ONNX runtime (CPU)."""
    import onnx_asr

    model = onnx_asr.load_model("nemo-parakeet-tdt-0.6b-v2")
    text = model.recognize(str(audio_path))

    return {"text": text, "sentences": [], "backend": "onnx"}


def transcribe_file(audio_path: Path, backend: str) -> dict:
    """Transcribe audio file using the specified backend."""
    if backend == "mlx":
        return transcribe_mlx(audio_path)
    elif backend == "nemo":
        return transcribe_nemo(audio_path)
    elif backend == "onnx":
        return transcribe_onnx(audio_path)
    else:
        raise ValueError(f"Unknown backend: {backend}")


def format_duration(seconds: float) -> str:
    """Format seconds as MM:SS."""
    mins = int(seconds // 60)
    secs = int(seconds % 60)
    return f"{mins}:{secs:02d}"


def main():
    # Parse arguments
    args = sys.argv[1:]

    if not args or args[0] in ("-h", "--help"):
        console.print("[bold]yt-text[/] - Local video transcription\n")
        console.print("Usage: uv run cli.py <url-or-file> [--backend mlx|nemo|onnx]\n")
        console.print("Examples:")
        console.print("  uv run cli.py https://youtube.com/watch?v=...")
        console.print("  uv run cli.py ~/Downloads/podcast.mp3")
        console.print("  uv run cli.py audio.wav --backend nemo")
        console.print("\nBackends:")
        console.print("  mlx   - Apple Silicon (parakeet-mlx)")
        console.print("  nemo  - NVIDIA GPU (nemo_toolkit)")
        console.print("  onnx  - CPU fallback (onnx-asr)")
        sys.exit(0)

    input_path = args[0]
    backend = None

    # Parse --backend flag
    if "--backend" in args:
        idx = args.index("--backend")
        if idx + 1 < len(args):
            backend = args[idx + 1]

    # Auto-detect backend if not specified
    if backend is None:
        backend = detect_backend()

    console.print(f"\n[dim]Backend:[/] {backend}")

    start_time = time.time()

    # Check if input is a file or URL
    path = Path(input_path).expanduser()
    is_file = path.exists() and path.is_file()

    with Progress(
        SpinnerColumn(),
        TextColumn("[progress.description]{task.description}"),
        console=console,
    ) as progress:
        with tempfile.TemporaryDirectory() as tmpdir:
            tmpdir_path = Path(tmpdir)

            if is_file:
                console.print(f"[dim]File:[/] {path.name}")

                # Convert to WAV if needed
                if path.suffix.lower() != ".wav":
                    task = progress.add_task("Converting to WAV...", total=None)
                    wav_path = tmpdir_path / "audio.wav"
                    convert_to_wav(path, wav_path)
                    audio_path = wav_path
                    progress.remove_task(task)
                else:
                    audio_path = path
            else:
                console.print(f"[dim]URL:[/] {input_path}")

                task = progress.add_task("Downloading audio...", total=None)
                audio_path = download_audio(input_path, tmpdir_path)
                progress.remove_task(task)

            task = progress.add_task(f"Transcribing with Parakeet ({backend})...", total=None)
            result = transcribe_file(audio_path, backend)
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

    # Show timestamps if available (MLX only currently)
    if result.get("sentences"):
        console.print("\n[bold]Timestamps:[/]")
        for s in result["sentences"][:5]:
            text_preview = s["text"][:60] + "..." if len(s["text"]) > 60 else s["text"]
            console.print(f"  [{format_duration(s['start'])}] {text_preview}")
        if len(result["sentences"]) > 5:
            console.print(f"  [dim]... and {len(result['sentences']) - 5} more[/]")


if __name__ == "__main__":
    main()
