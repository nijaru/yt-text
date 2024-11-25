import argparse
import json
import os
import sys
import traceback

import yt_dlp


def validate_url(url):
    result = {"valid": False, "duration": 0, "file_size": 0, "format": "", "error": ""}

    try:
        # Print debug info to stderr
        print(f"Validating URL: {url}", file=sys.stderr)

        if not url.startswith(("http://", "https://")):
            result["error"] = "URL must start with http:// or https://"
            return result

        ydl_opts = {
            "quiet": True,
            "no_warnings": True,
            "extract_flat": True,  # Only extract metadata
        }

        with yt_dlp.YoutubeDL(ydl_opts) as ydl:
            print("Extracting video info...", file=sys.stderr)
            info = ydl.extract_info(url, download=False)

            # Remove the noisy info dump
            # print(f"Extracted info: {json.dumps(info, indent=2)}", file=sys.stderr)

            if info is None:
                result["error"] = "Failed to extract information from the URL"
                return result

            # Check if the URL points to a single video
            if info.get("_type") and info["_type"] != "video":
                result["error"] = "URL must be for a single video"
                return result

            # Set video information
            result["valid"] = True
            result["duration"] = info.get("duration", 0)
            result["file_size"] = info.get("filesize", 0)
            result["format"] = info.get("ext", "")

    except yt_dlp.utils.DownloadError as e:
        result["error"] = f"Download error: {str(e)}"
        print(f"DownloadError: {str(e)}", file=sys.stderr)
        print(traceback.format_exc(), file=sys.stderr)
    except Exception as e:
        result["error"] = f"Validation error: {str(e)}"
        print(f"Unexpected error: {str(e)}", file=sys.stderr)
        print(traceback.format_exc(), file=sys.stderr)

    print(f"Validation result: {json.dumps(result, indent=2)}", file=sys.stderr)
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
