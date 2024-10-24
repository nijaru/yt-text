import argparse
import os
import sqlite3

import whisper
import yt_dlp


def initialize_db() -> sqlite3.Connection:
    conn = sqlite3.connect("urls.db")
    cursor = conn.cursor()
    cursor.execute("""
        CREATE TABLE IF NOT EXISTS urls (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            url TEXT NOT NULL,
            text TEXT NOT NULL
        )
    """)
    conn.commit()
    return conn


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
    except yt_dlp.DownloadError as e:
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


def save_to_db(conn, url, text) -> None:
    cursor = conn.cursor()
    try:
        cursor.execute(
            "INSERT INTO urls (url, text) VALUES (?, ?)",
            (url, text),
        )
        conn.commit()
    except sqlite3.Error as e:
        raise RuntimeError(f"Failed to save to database: {e}")


if __name__ == "__main__":
    parser = argparse.ArgumentParser(
        description="Download audio from youtube video and convert it to text"
    )
    parser.add_argument("url", type=str, help="URL of the youtube video")

    args = parser.parse_args()
    url = args.url

    conn = initialize_db()
    try:
        model_name = "tiny.en"
        audio = download_audio(url)
        text = transcribe_audio(audio, model_name)
        save_to_db(conn, url, text)
    finally:
        conn.close()
