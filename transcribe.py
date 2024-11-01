import argparse
import os
import random
import time

import whisper
import yt_dlp


def download_audio(url) -> str:
    ydl_opts = {
        "format": "bestaudio",
        "outtmpl": "%(id)s.%(ext)s",
        "quiet": True,
        "no_warnings": True,
    }
    try:
        with yt_dlp.YoutubeDL(ydl_opts) as ydl:
            info = ydl.extract_info(url, download=True)
            fname = ydl.prepare_filename(info)
        return fname
    except yt_dlp.utils.DownloadError as e:
        raise RuntimeError(f"Failed to download audio: {e}")
    except Exception as e:
        raise RuntimeError(f"An unexpected error occurred: {e}")


def transcribe_audio(file, model_name):
    try:
        model = whisper.load_model(model_name)
        results = model.transcribe(file)
        text = results["text"]
        os.remove(file)
        return text
    except Exception as e:
        raise RuntimeError(f"An unexpected error occurred during transcription: {e}")


def save_to_file(file, text) -> str:
    try:
        filename = f"{os.path.splitext(file)[0]}.txt"
        with open(filename, "w") as f:
            f.write(text)
        return filename
    except Exception as e:
        raise RuntimeError(f"Failed to save to file: {e}")


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

    args = parser.parse_args()
    url = args.url
    model_name = args.model

    filename = retry_with_backoff(lambda: download_audio(url))
    text = retry_with_backoff(lambda: transcribe_audio(filename, model_name))
    filename = retry_with_backoff(lambda: save_to_file(filename, text))
    print(filename)


if __name__ == "__main__":
    main()
