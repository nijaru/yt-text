import argparse
from pathlib import Path

import torch
from transcription import Transcriber


def format_transcript(text: str) -> str:
    """
    Formats the transcript by adding newlines after sentences and stripping whitespace.

    Args:
        text (str): The original transcript text.

    Returns:
        str: The formatted transcript text.
    """
    text = text.strip()
    for punct in [". ", "! ", "? "]:
        text = text.replace(punct, punct[0] + "\n")
    # Strip leading/trailing whitespace on each line
    formatted_lines = [line.strip() for line in text.split("\n")]
    return "\n".join(formatted_lines)


def process_urls(
    urls: list, audio_dir: Path, transcripts_dir: Path, transcriber: Transcriber
):
    """Process a list of URLs for transcription."""
    for url in urls:
        try:
            print(f"Processing URL: {url}")
            # Transcribe audio
            result = transcriber.process_url(url)
            if result.get("error"):
                print(f"Error processing {url}: {result['error']}")
                continue

            # Format transcription
            formatted_text = format_transcript(result["text"])

            # Determine video ID for file naming
            video_id = extract_video_id(url)
            transcript_filename = f"{video_id}.txt"
            transcript_path = transcripts_dir / transcript_filename
            with transcript_path.open("w", encoding="utf-8") as f:
                f.write(formatted_text)
            print(f"Saved transcript to {transcript_path}")

        except Exception as e:
            print(f"Unexpected error processing {url}: {e}")
            continue


def extract_video_id(url: str) -> str:
    """Extracts the video ID from a YouTube URL."""
    from urllib.parse import parse_qs, urlparse

    parsed_url = urlparse(url)
    if parsed_url.hostname in ["youtu.be"]:
        return parsed_url.path[1:]
    elif parsed_url.hostname in ["www.youtube.com", "youtube.com"]:
        query = parse_qs(parsed_url.query)
        return query.get("v", "")[0]
    else:
        return url  # Fallback to the full URL if parsing fails


def main():
    parser = argparse.ArgumentParser(
        description="Download and transcribe YouTube videos"
    )
    parser.add_argument(
        "urls",
        nargs="+",
        help="YouTube video URL(s). Can be individual URLs or comma-separated lists.",
    )
    args = parser.parse_args()

    # Parse URLs, allowing comma-separated lists
    urls = []
    for arg in args.urls:
        urls.extend([url.strip() for url in arg.split(",") if url.strip()])

    if not urls:
        print("No valid URLs provided.")
        return

    # Set up directories in the current working directory
    cwd = Path.cwd()
    transcripts_parent_dir = cwd / "youtube_transcripts"
    audio_dir = transcripts_parent_dir / "audio"
    transcripts_dir = transcripts_parent_dir / "transcripts"

    # Create necessary directories
    audio_dir.mkdir(parents=True, exist_ok=True)
    transcripts_dir.mkdir(parents=True, exist_ok=True)

    # Initialize Transcriber without constraints
    device = "cuda" if torch.cuda.is_available() else "cpu"
    transcriber = Transcriber(
        model_name="base.en",
        device=device,
        compute_type="float16" if device == "cuda" else "float32",
        max_video_duration=None,  # No constraints
        max_file_size=None,
    )

    try:
        process_urls(urls, audio_dir, transcripts_dir, transcriber)
    finally:
        # Clean up resources
        transcriber.close()
        print("Processing completed.")


if __name__ == "__main__":
    main()
