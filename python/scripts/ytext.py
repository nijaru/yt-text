import argparse
import hashlib
import json
import re
import shutil
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
    # Remove invalid characters
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


def get_expected_transcript_filename(url: str) -> str:
    """
    Generate a consistent and expected transcript filename based solely on the URL.

    This ensures that the filename can be determined without processing the URL.

    Args:
        url (str): The media URL.

    Returns:
        str: The expected transcript filename.
    """
    root_domain = extract_root_domain(url)
    short_hash = generate_short_unique_identifier(url)
    sanitized_title = f"{root_domain}_{short_hash}"
    return f"{sanitized_title}.txt"


def load_url_mapping(mapping_file: Path) -> dict:
    """
    Load the URL-to-filename mapping from a JSON file.

    Args:
        mapping_file (Path): Path to the mapping JSON file.

    Returns:
        dict: A dictionary mapping URLs to transcript filenames.
    """
    if mapping_file.exists():
        with mapping_file.open("r", encoding="utf-8") as f:
            return json.load(f)
    return {}


def save_url_mapping(mapping_file: Path, mapping: dict):
    """
    Save the URL-to-filename mapping to a JSON file.

    Args:
        mapping_file (Path): Path to the mapping JSON file.
        mapping (dict): The mapping dictionary to save.
    """
    with mapping_file.open("w", encoding="utf-8") as f:
        json.dump(mapping, f, indent=4)


def initialize_directories(base_dir: Path) -> tuple:
    """
    Initialize necessary directories for audio and transcripts.

    Args:
        base_dir (Path): The base directory where 'ytext_output' will be created.

    Returns:
        tuple: Paths to the audio directory and transcripts directory.
    """
    ytext_output_dir = base_dir / "ytext_output"
    audio_dir = ytext_output_dir / ".audio"
    transcripts_dir = ytext_output_dir

    # Create directories if they don't exist
    audio_dir.mkdir(parents=True, exist_ok=True)
    ytext_output_dir.mkdir(parents=True, exist_ok=True)  # Ensure root exists

    return audio_dir, transcripts_dir


def load_transcription_result(transcriber: Transcriber, url: str) -> dict:
    """
    Transcribe the media from the given URL.

    Args:
        transcriber (Transcriber): The transcriber instance.
        url (str): The media URL to transcribe.

    Returns:
        dict: Transcription result containing 'text' and optionally 'error' and 'title'.
    """
    result = transcriber.process_url(url)
    return result


def check_transcript_exists(url: str, transcripts_dir: Path, mapping: dict) -> tuple:
    """
    Check if a transcript already exists for the given URL.

    Args:
        url (str): The media URL.
        transcripts_dir (Path): Directory where transcripts are stored.
        mapping (dict): The URL-to-filename mapping dictionary.

    Returns:
        tuple: (exists: bool, transcript_path: Path or None)
    """
    if url in mapping:
        existing_transcript = transcripts_dir / mapping[url]
        if existing_transcript.exists():
            print(f"Transcript already exists for {url}: {existing_transcript}")
            return True, existing_transcript
        else:
            print(f"Mapping exists but file missing for {url}. Reprocessing.")

    else:
        expected_filename = get_expected_transcript_filename(url)
        expected_path = transcripts_dir / expected_filename

        if expected_path.exists():
            print(f"Transcript already exists for {url}: {expected_path}")
            mapping[url] = expected_filename  # Update mapping
            return True, expected_path

    return False, None


def determine_transcript_filename(url: str, result: dict) -> str:
    """
    Determine the filename for the transcript based on the media title or URL.

    Args:
        url (str): The media URL.
        result (dict): Transcription result containing 'title'.

    Returns:
        str: The transcript filename.
    """
    media_title = result.get("title")
    if media_title and media_title.lower() != url.lower():
        sanitized_title = sanitize_filename(media_title)
    else:
        sanitized_title = get_expected_transcript_filename(url).replace(".txt", "")

    return f"{sanitized_title}.txt"


def save_transcript(transcript_path: Path, formatted_text: str) -> None:
    """
    Save the formatted transcript to a file.

    Args:
        transcript_path (Path): Path where the transcript will be saved.
        formatted_text (str): The formatted transcript text.
    """
    with transcript_path.open("w", encoding="utf-8") as f:
        f.write(formatted_text)
    print(f"Saved transcript to {transcript_path}")


def update_mapping(mapping: dict, url: str, transcript_filename: str) -> None:
    """
    Update the URL-to-filename mapping dictionary.

    Args:
        mapping (dict): The URL-to-filename mapping dictionary.
        url (str): The media URL.
        transcript_filename (str): The transcript filename.
    """
    mapping[url] = transcript_filename


def process_single_url(
    url: str, transcripts_dir: Path, transcriber: Transcriber, mapping: dict
) -> tuple:
    """
    Process a single URL: check if transcription exists, transcribe if necessary, and save the transcript.

    Args:
        url (str): The media URL to process.
        transcripts_dir (Path): Directory where transcripts are stored.
        transcriber (Transcriber): The transcriber instance.
        mapping (dict): The URL-to-filename mapping dictionary.

    Returns:
        tuple: (status, message)
            status: 'skipped', 'success', or 'failed'
            message: Additional information
    """
    try:
        print(f"Processing URL: {url}")

        # Step 1: Check if transcript exists
        exists, transcript_path = check_transcript_exists(url, transcripts_dir, mapping)
        if exists:
            return ("skipped", f"Transcript exists at {transcript_path}")

        # Step 2: Transcribe the URL
        result = load_transcription_result(transcriber, url)
        if result.get("error"):
            error_msg = result["error"]
            print(f"Error processing {url}: {error_msg}")
            return ("failed", error_msg)

        # Step 3: Format the transcript
        formatted_text = format_transcript(result["text"])

        # Step 4: Determine transcript filename
        transcript_filename = determine_transcript_filename(url, result)
        transcript_path = transcripts_dir / transcript_filename

        # Step 5: Save the transcript
        save_transcript(transcript_path, formatted_text)

        # Step 6: Update the mapping
        update_mapping(mapping, url, transcript_filename)

        return ("success", f"Transcript saved at {transcript_path}")

    except Exception as e:
        print(f"Unexpected error processing {url}: {e}")
        return ("failed", str(e))


def process_urls(
    urls: list, transcripts_dir: Path, transcriber: Transcriber, mapping: dict
) -> dict:
    """
    Process a list of URLs for transcription.

    Args:
        urls (list): List of media URLs to process.
        transcripts_dir (Path): Directory where transcripts are stored.
        transcriber (Transcriber): The transcriber instance.
        mapping (dict): The URL-to-filename mapping dictionary.

    Returns:
        dict: Summary of processing outcomes.
    """
    summary = {
        "total": len(urls),
        "processed": 0,
        "skipped": 0,
        "success": 0,
        "failed": 0,
        "failures": [],
    }

    for url in urls:
        status, message = process_single_url(url, transcripts_dir, transcriber, mapping)
        if status == "skipped":
            summary["skipped"] += 1
        elif status == "success":
            summary["processed"] += 1
            summary["success"] += 1
        elif status == "failed":
            summary["processed"] += 1
            summary["failed"] += 1
            summary["failures"].append({"url": url, "error": message})

    return summary


def parse_arguments() -> argparse.Namespace:
    """
    Parse command-line arguments.

    Returns:
        argparse.Namespace: The parsed arguments.
    """
    parser = argparse.ArgumentParser(description="Download and transcribe media")
    parser.add_argument(
        "--url",
        type=str,
        help="A single media URL to transcribe.",
    )
    parser.add_argument(
        "urls",
        nargs="*",
        help="Additional media URLs to transcribe.",
    )
    parser.add_argument(
        "--model",
        type=str,
        default="large-v3-turbo",
        help="Whisper model to use for transcription (e.g., 'large-v3-turbo', 'medium', 'small').",
    )
    parser.add_argument(
        "--prompt",
        type=str,
        help="Initial prompt to guide transcription (helpful for technical content).",
    )
    parser.add_argument(
        "--chunk-length",
        type=int,
        default=120,
        help="Length in seconds for chunking long videos (default: 120)",
    )
    return parser.parse_args()


def get_urls(args: argparse.Namespace) -> list:
    """
    Gather URLs from --url and positional arguments.

    Args:
        args (argparse.Namespace): Parsed command-line arguments.

    Returns:
        list: List of URLs.
    """
    urls = []
    if args.url:
        urls.append(args.url)
    for arg_url in args.urls:
        urls.extend([url.strip() for url in arg_url.split(",") if url.strip()])

    if not urls:
        print("No valid URLs provided.")
        return []
    return urls


def initialize_transcriber(args: argparse.Namespace) -> Transcriber:
    """
    Initialize the Transcriber with the specified options.

    Args:
        args (argparse.Namespace): Command line arguments containing model and options.

    Returns:
        Transcriber: Initialized Transcriber instance.
    """
    return Transcriber(
        model_name=args.model,
        device=None,  # Will be auto-detected based on hardware
        compute_type=None,  # Will be auto-optimized
        max_video_duration=None,
        max_file_size=None,
        chunk_length_seconds=args.chunk_length,
        initial_prompt=args.prompt,
    )


def cleanup_audio_directory(audio_dir: Path) -> None:
    """
    Clean up the audio directory by deleting all its contents.

    Args:
        audio_dir (Path): Path to the audio directory.
    """
    if audio_dir.exists() and audio_dir.is_dir():
        try:
            shutil.rmtree(audio_dir)
        except Exception as e:
            print(f"Failed to clean up audio directory {audio_dir}: {e}")
    else:
        print(f"Audio directory does not exist or is not a directory: {audio_dir}")


def print_summary(summary: dict) -> None:
    """
    Print a summary of processing outcomes.

    Args:
        summary (dict): Summary dictionary containing counts and failure details.
    """
    print("\n=== Processing Summary ===")
    print(f"Total URLs: {summary['total']}")
    print(f"Processed: {summary['processed']}")
    print(f" - Successful: {summary['success']}")
    print(f" - Failed: {summary['failed']}")
    print(f"Skipped (transcript already exists): {summary['skipped']}")
    if summary["failures"]:
        print("\n--- Failures ---")
        for failure in summary["failures"]:
            print(f"URL: {failure['url']}\nError: {failure['error']}\n")
    print("==========================\n")


def main():
    args = parse_arguments()

    urls = get_urls(args)
    if not urls:
        return

    # Initialize directories
    cwd = Path.cwd()
    audio_dir, transcripts_dir = initialize_directories(cwd)

    # Initialize URL mapping
    mapping_file = transcripts_dir / ".ytext_url_mapping.json"  # Hidden file
    mapping = load_url_mapping(mapping_file)

    # Initialize Transcriber with the specified options
    transcriber = initialize_transcriber(args)

    summary = {}

    try:
        # Process URLs
        summary = process_urls(urls, transcripts_dir, transcriber, mapping)
    finally:
        # Save updated URL mappings
        save_url_mapping(mapping_file, mapping)
        # Clean up resources
        transcriber.close()
        # Clean up audio directory
        cleanup_audio_directory(audio_dir)
        # Print summary
        print_summary(summary)
        print("Processing completed.")


if __name__ == "__main__":
    main()
