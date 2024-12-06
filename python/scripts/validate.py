import argparse
import json
import sys
import traceback

import yt_dlp

MAX_VIDEO_DURATION = 4 * 3600  # 4 hours in seconds


def validate_url(url: str) -> dict:
    result = {
        "valid": False,
        "duration": 0,
        "format": "",
        "error": "",
    }

    try:
        with yt_dlp.YoutubeDL({"quiet": True}) as ydl:
            info = ydl.extract_info(url, download=False)

            duration = info.get("duration", 0)
            if duration > MAX_VIDEO_DURATION:
                result["error"] = (
                    f"Video too long: {duration} seconds (max: {MAX_VIDEO_DURATION} seconds)"
                )
                return result

            # Additional validation can be added here (e.g., supported formats)

            result.update(
                {
                    "valid": True,
                    "duration": duration,
                    "format": info.get("ext", ""),
                }
            )

    except yt_dlp.utils.DownloadError as e:
        result["error"] = f"Download error: {str(e)}"
    except Exception as e:
        result["error"] = f"Unexpected error: {str(e)}"

    return result


def main():
    parser = argparse.ArgumentParser(description="Validate Media URL")
    parser.add_argument("--url", type=str, required=True, help="URL to validate")
    args = parser.parse_args()

    try:
        result = validate_url(args.url)
        print(json.dumps(result))

        if not result["valid"]:
            sys.exit(1)

    except Exception as e:
        print(f"Critical error: {str(e)}", file=sys.stderr)
        print(traceback.format_exc(), file=sys.stderr)
        sys.exit(2)


if __name__ == "__main__":
    main()
