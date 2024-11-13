import argparse
import json
import os
import random
import subprocess
import tempfile
import time

import whisper
import yt_dlp


def check_dependencies():
    try:
        subprocess.run(["ffmpeg", "-version"], capture_output=True, check=True)
    except (subprocess.SubprocessError, FileNotFoundError):
        raise RuntimeError("ffmpeg is not installed or not accessible")


def check_video_length(url) -> float:
    ydl_opts = {
        "quiet": True,
        "no_warnings": True,
    }
    with yt_dlp.YoutubeDL(ydl_opts) as ydl:
        info = ydl.extract_info(url, download=False)
        duration = info.get("duration", 0)  # Duration in seconds
        return duration


def check_file_size(url) -> int:
    ydl_opts = {
        "quiet": True,
        "no_warnings": True,
    }
    with yt_dlp.YoutubeDL(ydl_opts) as ydl:
        info = ydl.extract_info(url, download=False)
        filesize = info.get("filesize", 0)
        if filesize == 0:
            filesize = info.get("filesize_approx", 0)
        return filesize


def download_audio(url) -> str:
    # Create a temporary directory in /tmp with explicit permissions
    temp_dir = tempfile.mkdtemp(dir="/tmp")
    os.chmod(temp_dir, 0o755)

    ydl_opts = {
        "format": "bestaudio",
        "outtmpl": os.path.join(temp_dir, "%(id)s.%(ext)s"),
        "quiet": True,
        "no_warnings": True,
        "cachedir": False,  # Disable cache
    }
    try:
        with yt_dlp.YoutubeDL(ydl_opts) as ydl:
            info = ydl.extract_info(url, download=True)
            fname = ydl.prepare_filename(info)
            # Ensure the downloaded file is readable
            os.chmod(fname, 0o644)
            return fname
    except yt_dlp.utils.DownloadError as e:
        if os.path.exists(temp_dir):
            import shutil

            shutil.rmtree(temp_dir)
        raise RuntimeError(f"Failed to download audio: {e}")
    except Exception as e:
        if os.path.exists(temp_dir):
            import shutil

            shutil.rmtree(temp_dir)
        raise RuntimeError(f"An unexpected error occurred: {e}")


def transcribe_audio(file, model_name):
    try:
        # Use environment variable to set cache directory to /tmp
        os.environ["TRANSFORMERS_CACHE"] = "/tmp"
        os.environ["HF_HOME"] = "/tmp"
        os.environ["XDG_CACHE_HOME"] = "/tmp"
        os.environ["TORCH_HOME"] = "/tmp"

        model = whisper.load_model(model_name)
        results = model.transcribe(file)
        text = results["text"]
        # File will be automatically cleaned up when the temporary directory is removed
        return text
    except Exception as e:
        raise RuntimeError(f"An unexpected error occurred during transcription: {e}")


def retry_with_backoff(
    func, max_retries=3, initial_backoff=2, max_backoff=30, backoff_factor=2.0
):
    for attempt in range(1, max_retries + 1):
        try:
            return func()
        except Exception:
            if attempt == max_retries:
                raise
            backoff = min(
                initial_backoff * (backoff_factor ** (attempt - 1)), max_backoff
            )
            time.sleep(backoff + random.uniform(0, backoff / 2))


def main():
    parser = argparse.ArgumentParser(
        description="Download audio from youtube video and convert it to text"
    )
    parser.add_argument("url", type=str, help="URL of the youtube video")
    parser.add_argument(
        "--model", type=str, default="base.en", help="Name of the Whisper model to use"
    )
    parser.add_argument(
        "--json", action="store_true", help="Return the transcription as a JSON object"
    )

    args = parser.parse_args()
    url = args.url
    model_name = args.model
    return_json = args.json

    # Check video length (1 hour = 3600 seconds)
    duration = check_video_length(url)
    if duration > 3600:
        raise ValueError("Video is longer than 1 hour")

    # Check file size (100MB limit)
    MAX_SIZE = 100 * 1024 * 1024  # 100MB in bytes
    size = check_file_size(url)
    if size > MAX_SIZE:
        raise ValueError("File size too large")

    # Use a context manager for the temporary directory
    with tempfile.TemporaryDirectory(dir="/tmp") as temp_dir:
        filename = retry_with_backoff(lambda: download_audio(url))
        text = retry_with_backoff(lambda: transcribe_audio(filename, model_name))

        if return_json:
            response = {"transcription": text, "model_name": model_name}
            print(json.dumps(response))
        else:
            # Save to temp file if needed
            temp_output = os.path.join(
                temp_dir, f"{os.path.splitext(os.path.basename(filename))[0]}.txt"
            )
            with open(temp_output, "w") as f:
                f.write(text)
            print(temp_output)


if __name__ == "__main__":
    check_dependencies()
    main()
