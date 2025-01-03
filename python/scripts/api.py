import argparse
import json
import sys

from transcription import Transcriber


def main():
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
        error_response = {
            "text": None,
            "model_name": args.model,
            "duration": 0,
            "error": "No valid URLs provided.",
            "title": None,
            "url": None,
        }
        print(json.dumps(error_response))
        sys.exit(1)

    # If constraints are enabled, limit to one URL
    if args.enable_constraints:
        urls = urls[:1]

    formatted_result = None

    try:
        transcriber = Transcriber(
            model_name=args.model,
            max_video_duration=4 * 3600 if args.enable_constraints else None,
            max_file_size=100 * 1024 * 1024 if args.enable_constraints else None,
        )

        results = []
        for url in urls:
            result = transcriber.process_url(url)
            results.append(result)

        transcriber.close()

        # Prepare the final result
        if len(results) == 1:
            output = results[0]
        else:
            output = results

        # Format the output to include only necessary fields
        if isinstance(output, dict):
            formatted_result = {
                "text": output.get("text"),
                "model_name": output.get("model_name"),
                "duration": output.get("duration", 0),
                "error": output.get("error"),
                "title": output.get("title"),
                "url": output.get("url"),
            }
        else:
            # Handle list of results
            formatted_result = []
            for item in output:
                formatted_item = {
                    "text": item.get("text"),
                    "model_name": item.get("model_name"),
                    "duration": item.get("duration", 0),
                    "error": item.get("error"),
                    "title": item.get("title"),
                    "url": item.get("url"),
                }
                formatted_result.append(formatted_item)

    except Exception as e:
        # Standardize error output format
        formatted_result = {
            "text": None,
            "model_name": args.model,
            "duration": 0,
            "error": f"Unexpected error: {e}",
            "title": None,
            "url": urls[0] if urls else None,
        }

    finally:
        # Output JSON response without any logging
        sys.stdout.write(json.dumps(formatted_result))
        sys.stdout.flush()


if __name__ == "__main__":
    main()
