import argparse
import hashlib
import re
from pathlib import Path
from urllib.parse import urlparse

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


def sanitize_filename(name: str) -> str:
    """
    Sanitize the filename by removing or replacing invalid characters and limiting its length.

    Args:
        name (str): The original filename.

    Returns:
        str: A sanitized filename safe for most filesystems.
    """
    # Remove any character that is not alphanumeric, space, hyphen, or underscore
    sanitized = re.sub(r'[<>:"/\\|?*]', "", name)
    sanitized = sanitized.strip().replace(" ", "_")
    # Limit filename length to 100 characters
    return sanitized[:100] if len(sanitized) > 100 else sanitized


def generate_short_unique_identifier(url: str, length: int = 8) -> str:
    """
    Generate a short unique identifier based on the URL.

    Args:
        url (str): The media URL.
        length (int): Desired length of the unique identifier.

    Returns:
        str: A shortened unique identifier string.
    """
    # Use a hash of the URL and take the first 'length' characters
    return hashlib.md5(url.encode("utf-8")).hexdigest()[:length]


def extract_root_domain(url: str) -> str:
    """
    Extract the root domain from a URL.

    Args:
        url (str): The media URL.

    Returns:
        str: The root domain (e.g., 'instagram' from 'instagram.com/post/xyz').
    """
    try:
        parsed_url = urlparse(url)
        domain = parsed_url.netloc.lower()
        # Remove 'www.' prefix if present
        if domain.startswith("www."):
            domain = domain[4:]
        # Extract the first part of the domain
        root_domain = domain.split(".")[0]
        return root_domain
    except Exception:
        return "media"


def process_urls(
    urls: list, audio_dir: Path, transcripts_dir: Path, transcriber: Transcriber
):
    """Process a list of URLs for transcription."""
    for url in urls:
        try:
            print(f"Processing URL: {url}")
            # Process URL and get transcription result
            result = transcriber.process_url(url)
            if result.get("error"):
                print(f"Error processing {url}: {result['error']}")
                continue

            # Format transcription
            formatted_text = format_transcript(result["text"])

            # Extract media title for file naming
            media_title = result.get("title")
            if media_title and media_title.lower() != url.lower():
                sanitized_title = sanitize_filename(media_title)
            else:
                # Fallback: Use root domain with short unique identifier
                root_domain = extract_root_domain(url)
                short_hash = generate_short_unique_identifier(url)
                sanitized_title = f"{root_domain}_{short_hash}"

            transcript_filename = f"{sanitized_title}.txt"
            transcript_path = transcripts_dir / transcript_filename
            with transcript_path.open("w", encoding="utf-8") as f:
                f.write(formatted_text)
            print(f"Saved transcript to {transcript_path}")

        except Exception as e:
            print(f"Unexpected error processing {url}: {e}")
            continue


def main():
    parser = argparse.ArgumentParser(description="Download and transcribe media")
    parser.add_argument(
        "urls",
        nargs="+",
        help="Media URL(s). Can be individual URLs or comma-separated lists.",
    )
    # Add the --model argument here
    parser.add_argument(
        "--model",
        type=str,
        default="base.en",
        help="Whisper model to use for transcription (e.g., 'base.en', 'small', 'medium', 'large').",
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
    transcripts_parent_dir = cwd / "media_transcripts"
    audio_dir = transcripts_parent_dir / "audio"
    transcripts_dir = transcripts_parent_dir / "transcripts"

    # Create necessary directories
    audio_dir.mkdir(parents=True, exist_ok=True)
    transcripts_dir.mkdir(parents=True, exist_ok=True)

    # Initialize Transcriber with the specified model
    device = "cuda" if torch.cuda.is_available() else "cpu"
    transcriber = Transcriber(
        model_name=args.model,  # Use the model name from the argument
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
