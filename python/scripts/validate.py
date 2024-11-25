import argparse
import json
import os
import sys
import traceback

import yt_dlp

MAX_VIDEO_DURATION = 4 * 3600  # 4 hours in seconds


def validate_url(url: str) -> dict:
    result = {"valid": False, "duration": 0, "file_size": 0, "format": "", "error": ""}

    try:
        with yt_dlp.YoutubeDL({"quiet": True}) as ydl:
            info = ydl.extract_info(url, download=False)

            duration = info.get("duration", 0)
            if duration > MAX_VIDEO_DURATION:
                result["error"] = (
                    f"Video too long: {duration} seconds (max: {MAX_VIDEO_DURATION})"
                )
                return result

            result.update(
                {
                    "valid": True,
                    "duration": duration,
                    "format": info.get("ext", ""),
                }
            )

    except Exception as e:
        result["error"] = str(e)

    return result


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description="Validate YouTube URL")
    parser.add_argument("--url", type=str, required=True, help="URL to validate")
    parser.add_argument("--json", action="store_true", help="Output in JSON format")
    args = parser.parse_args()

    try:
        print(f"Starting validation for URL: {args.url}", file=sys.stderr)
        print(
            f"Script directory: {os.path.dirname(os.path.abspath(__file__))}",
            file=sys.stderr,
        )
        print(f"Current working directory: {os.getcwd()}", file=sys.stderr)
        print(f"Environment: {os.environ}", file=sys.stderr)

        result = validate_url(args.url)
        print(json.dumps(result))

        if not result["valid"]:
            sys.exit(1)

    except Exception as e:
        print(f"Critical error: {str(e)}", file=sys.stderr)
        print(traceback.format_exc(), file=sys.stderr)
        sys.exit(2)
