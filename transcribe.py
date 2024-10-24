import argparse
import os

import whisper
import yt_dlp


def download_audio(url) -> str:
    ydl_opts = {
        "format": "bestaudio",
        "outtmpl": "%(id)s.%(ext)s",
        # "sponsorblock": ["all"],
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


def transcribe_audio(file):
    try:
        model = whisper.load_model("tiny.en")
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


if __name__ == "__main__":
    parser = argparse.ArgumentParser(
        description="Download audio from youtube video and convert it to text"
    )
    parser.add_argument("url", type=str, help="URL of the youtube video")
    args = parser.parse_args()
    url = args.url

    model_name = "tiny.en"
    filename = download_audio(url)
    text = transcribe_audio(filename)
    filename = save_to_file(filename, text)
    print(filename)
