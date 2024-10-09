import os
import sqlite3

import whisper
import yt_dlp


def initialize_db() -> sqlite3.Connection:
    conn = sqlite3.connect("transcriptions.db")
    cursor = conn.cursor()
    cursor.execute("""
        CREATE TABLE IF NOT EXISTS transcriptions (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            url TEXT NOT NULL,
            text TEXT NOT NULL,
            model_name TEXT NOT NULL
        )
    """)
    conn.commit()
    return conn


def check_db(url, conn) -> sqlite3.Row:
    cursor = conn.cursor()
    cursor.execute("SELECT text FROM transcriptions WHERE url = ?", (url,))
    row = cursor.fetchone()
    return row


def transcribe(url, conn, model_name="tiny.en") -> None:
    audio = download_audio(url)
    text = transcribe_audio(audio, model_name)
    save_to_db(conn, url, text, model_name)


def download_audio(url) -> str:
    ydl_opts = {
        "format": "bestaudio",
        "outtmpl": "%(id)s.%(ext)s",
        "sponsorblock": ["all"],
        "quiet": True,
        "no_warnings": True,
    }
    with yt_dlp.YoutubeDL(ydl_opts) as ydl:
        info = ydl.extract_info(url, download=True)
        fname = ydl.prepare_filename(info)
    return fname


def transcribe_audio(file, model_name):
    model = whisper.load_model(model_name)
    results = model.transcribe(file)
    text = results["text"]
    os.remove(file)
    return text


def save_to_db(conn, url, text, model_name) -> None:
    cursor = conn.cursor()
    cursor.execute(
        "INSERT INTO transcriptions (url, text, model_name) VALUES (?, ?, ?)",
        (url, text, model_name),
    )
    conn.commit()


def main():
    import argparse

    parser = argparse.ArgumentParser(
        description="Download audio from youtube video and convert it to text"
    )
    parser.add_argument("url", type=str, help="URL of the youtube video")
    # add verbose mode which prints text after transcription
    parser.add_argument(
        "-v", "--verbose", action="store_true", help="Output text after transcription"
    )
    args = parser.parse_args()
    url = args.url
    verbose = args.verbose

    conn = initialize_db()

    try:
        row = check_db(url, conn)
        if row:
            if verbose:
                print(row[0])
            else:
                print("Transcription already exists.")
        else:
            transcribe(url, conn)
            if verbose:
                print(check_db(url, conn)[0])
            else:
                print("Transcription completed.")
    finally:
        conn.close()


if __name__ == "__main__":
    main()
