import argparse
import json
import os
import re
import sys
import tempfile
import traceback
from typing import Dict, List

import yt_dlp
from webvtt import WebVTT


class NullLogger:
    def debug(self, msg):
        pass

    def warning(self, msg):
        pass

    def error(self, msg):
        pass


def get_subtitles(url: str) -> dict:
    """Extract available subtitles for a video."""
    result = {
        "success": False,
        "subtitles": {},
        "auto_subtitles": {},
        "title": None,
        "error": None,
    }

    ydl_opts = {
        "quiet": True,
        "no_warnings": True,
        "logger": NullLogger(),
    }

    try:
        with yt_dlp.YoutubeDL(ydl_opts) as ydl:
            info = ydl.extract_info(url, download=False)

            if not isinstance(info, dict):
                raise Exception("Failed to extract video information")

            # Format subtitle information
            def format_tracks(subs: Dict) -> Dict[str, List[Dict]]:
                formatted = {}
                for lang, tracks in subs.items():
                    # Filter out empty or invalid tracks
                    valid_tracks = [track for track in tracks if track.get("url")]
                    if valid_tracks:
                        formatted[lang] = [
                            {
                                "url": track.get("url"),
                                "language": lang,
                                "ext": track.get("ext", "vtt"),
                            }
                            for track in valid_tracks
                        ]
                return formatted

            result.update(
                {
                    "success": True,
                    "subtitles": format_tracks(info.get("subtitles", {})),
                    "auto_subtitles": format_tracks(info.get("automatic_captions", {})),
                    "title": info.get("title"),
                }
            )

    except Exception as e:
        result["error"] = str(e)

    return result


def clean_subtitles(text: str) -> str:
    """Clean and format WebVTT subtitles with robust error handling."""
    try:
        # Normalize line endings and ensure WEBVTT header
        text = text.replace("\r\n", "\n").replace("\r", "\n")
        if not text.strip().startswith("WEBVTT"):
            text = "WEBVTT\n\n" + text

        # Create temporary file for WebVTT parsing
        with tempfile.NamedTemporaryFile(mode="w", suffix=".vtt", delete=False) as tmp:
            tmp.write(text)
            tmp_path = tmp.name

        try:
            captions = WebVTT().read(tmp_path)

            # Extract and clean lines
            lines = []
            seen = set()

            for caption in captions:
                if not caption.text.strip():
                    continue

                for line in caption.text.split("\n"):
                    cleaned = clean_line(line)
                    if cleaned and cleaned not in seen:
                        seen.add(cleaned)
                        lines.append(cleaned)

            # Join lines with proper spacing
            result = " ".join(lines)
            result = re.sub(r"\s+", " ", result).strip()

        finally:
            # Clean up temporary file
            try:
                os.unlink(tmp_path)
            except OSError:
                pass

        return result if result else fallback_clean(text)

    except Exception as e:
        print(f"Subtitle cleaning error: {e}", file=sys.stderr)
        return fallback_clean(text)


def clean_line(line: str) -> str:
    """Clean a single subtitle line."""
    # Remove common patterns
    line = re.sub(r"<[^>]+>", "", line)  # HTML tags
    line = re.sub(r"\[.*?\]", "", line)  # [Music], [Applause], etc
    line = re.sub(r"^\s*>\s*", "", line)  # Leading >
    line = re.sub(r"{\\\w+}", "", line)  # SSA/ASS tags
    line = re.sub(r"\([^)]*\)", "", line)  # (text in parentheses)

    # Remove timing information
    line = re.sub(r"\d{2}:\d{2}:\d{2}[.,]\d{3}.*-->", "", line)

    return line.strip()


def fallback_clean(text: str) -> str:
    """Fallback cleaning method when WebVTT parsing fails."""
    # Remove headers
    text = re.sub(r"WEBVTT.*\n", "", text)

    # Remove timing lines
    text = re.sub(r"\d{2}:\d{2}:\d{2}[.,]\d{3}.*-->", "", text)

    # Split into lines and clean each
    lines = []
    seen = set()

    for line in text.split("\n"):
        cleaned = clean_line(line)
        if cleaned and cleaned not in seen:
            seen.add(cleaned)
            lines.append(cleaned)

    return " ".join(lines).strip()


def download_subtitle(url: str, lang: str, auto: bool = False) -> dict:
    """Download and clean subtitles with better error handling."""
    result = {"success": False, "text": None, "error": None}

    with tempfile.TemporaryDirectory() as temp_dir:
        ydl_opts = {
            "quiet": True,
            "no_warnings": True,
            "logger": NullLogger(),
            "writesubtitles": not auto,
            "writeautomaticsub": auto,
            "subtitleslangs": [lang],
            "skip_download": True,
            "subtitlesformat": "vtt",
            "outtmpl": os.path.join(temp_dir, "%(id)s"),
        }

        try:
            with yt_dlp.YoutubeDL(ydl_opts) as ydl:
                info = ydl.extract_info(url, download=True)
                if not isinstance(info, dict):
                    raise ValueError("Failed to extract video information")

                video_id = info["id"]
                patterns = [
                    f"{video_id}.{lang}.vtt",
                    f"{video_id}.{lang}-auto.vtt",
                    f"{video_id}.{lang}-orig.vtt",
                    f"{video_id}.{lang}-generated.vtt",
                ]

                # Find subtitle file
                sub_file = None
                for pattern in patterns:
                    path = os.path.join(temp_dir, pattern)
                    if os.path.exists(path):
                        sub_file = path
                        break

                if not sub_file:
                    result["error"] = "No subtitle file found"
                    return result

                # Read and clean subtitles
                with open(sub_file, "r", encoding="utf-8") as f:
                    subtitle_text = f.read()
                    if not subtitle_text.strip():
                        result["error"] = "Empty subtitle file"
                        return result

                    cleaned_text = clean_subtitles(subtitle_text)
                    if not cleaned_text:
                        result["error"] = "No valid subtitle content after cleaning"
                        return result

                    result["success"] = True
                    result["text"] = cleaned_text

        except Exception as e:
            result["error"] = str(e)
            print(
                f"Subtitle download error: {e}\n{traceback.format_exc()}",
                file=sys.stderr,
            )

    return result


def main():
    parser = argparse.ArgumentParser(description="Extract video subtitles")
    parser.add_argument("--url", type=str, required=True, help="Video URL")
    parser.add_argument("--download", action="store_true", help="Download subtitles")
    parser.add_argument("--lang", type=str, default="en", help="Subtitle language")
    parser.add_argument(
        "--auto", action="store_true", help="Use auto-generated subtitles"
    )

    args = parser.parse_args()

    if args.download:
        result = download_subtitle(args.url, args.lang, args.auto)
    else:
        result = get_subtitles(args.url)

    print(json.dumps(result))
    sys.stdout.flush()


if __name__ == "__main__":
    main()
