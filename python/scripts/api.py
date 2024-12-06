import argparse
import json
import logging
import sys

from transcription import Transcriber


def main():
    # Configure logging
    logging.basicConfig(
        level=logging.INFO,
        format="%(asctime)s [%(levelname)s] %(message)s",
        handlers=[logging.StreamHandler(sys.stdout)],
    )

    parser = argparse.ArgumentParser(description="Transcribe media")
    parser.add_argument(
        "--url", type=str, required=True, help="Media URL(s), comma-separated"
    )
    parser.add_argument("--model", default="base.en", help="Whisper model to use")
    parser.add_argument(
        "--enable_constraints",
        action="store_true",
        help="Enable video duration and file size constraints",
    )
    args = parser.parse_args()

    # Split URLs by comma and clean whitespace
    urls = [url.strip() for url in args.url.split(",") if url.strip()]

    if not urls:
        logging.warning("No valid URLs provided.")
        return

    # If constraints are enabled, limit to one URL
    if args.enable_constraints:
        urls = urls[:1]

    # Configuration for constraints
    max_video_duration = 4 * 3600 if args.enable_constraints else None
    max_file_size = 100 * 1024 * 1024 if args.enable_constraints else None

    # Initialize Transcriber
    transcriber = Transcriber(
        model_name=args.model,
        max_video_duration=max_video_duration,
        max_file_size=max_file_size,
    )

    results = []
    for url in urls:
        logging.info(f"Processing URL: {url}")
        result = transcriber.process_url(url)
        results.append(result)

    transcriber.close()

    # If only one URL was processed, return a single object instead of a list
    output = results[0] if len(results) == 1 else results

    # Output results as JSON
    print(json.dumps(output))


if __name__ == "__main__":
    main()
