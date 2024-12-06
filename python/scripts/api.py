import argparse
import json

from transcription import Transcriber


def main():
    parser = argparse.ArgumentParser(description="Transcribe YouTube video")
    parser.add_argument(
        "--url", type=str, required=True, help="YouTube video URL(s), comma-separated"
    )
    parser.add_argument("--model", default="base.en", help="Whisper model to use")
    parser.add_argument(
        "--enable_constraints",
        action="store_true",
        help="Enable video duration and file size constraints",
    )
    args = parser.parse_args()

    urls = [url.strip() for url in args.url.split(",") if url.strip()]

    # Limit to one URL if constraints are enabled
    urls = urls[:1] if args.enable_constraints else urls

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
        result = transcriber.process_url(url)
        results.append(result)

    transcriber.close()

    # Output results as JSON
    print(json.dumps(results))


if __name__ == "__main__":
    main()
