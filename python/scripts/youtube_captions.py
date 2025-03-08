#!/usr/bin/env python3
"""
YouTube Captions API Client.

This script fetches captions from YouTube Data API v3.
It retrieves caption tracks, downloads them, and converts them to plain text.
"""

import argparse
import html
import json
import logging
import sys
from typing import Any

import requests
from bs4 import BeautifulSoup

# Configure logging to use stderr instead of stdout
logging.basicConfig(level=logging.INFO, format="%(asctime)s - %(levelname)s - %(message)s", stream=sys.stderr)
logger = logging.getLogger(__name__)


class YouTubeCaptionClient:
    """Client for fetching captions from YouTube API."""

    def __init__(self, api_key: str):
        """Initialize client with API key.

        Args:
            api_key: YouTube Data API v3 key
        """
        self.api_key = api_key
        self.base_url = "https://www.googleapis.com/youtube/v3"

    def get_video_info(self, video_id: str) -> dict[str, Any]:
        """Get video metadata.

        Args:
            video_id: YouTube video ID

        Returns:
            Dict containing video information
        """
        url = f"{self.base_url}/videos"
        params = {"part": "snippet", "id": video_id, "key": self.api_key}

        try:
            response = requests.get(url, params=params)
            response.raise_for_status()
            data = response.json()

            if not data.get("items"):
                return {"error": "Video not found"}

            video_info = data["items"][0]["snippet"]
            return {
                "title": video_info.get("title", ""),
                "language": video_info.get("defaultAudioLanguage", ""),
            }
        except requests.RequestException as e:
            logger.exception(f"Error fetching video info: {str(e)}")
            return {"error": f"API error: {str(e)}"}

    def list_caption_tracks(self, video_id: str) -> list[dict[str, Any]]:
        """List available caption tracks for a video.

        Args:
            video_id: YouTube video ID

        Returns:
            List of caption track information
        """
        url = f"{self.base_url}/captions"
        params = {"part": "snippet", "videoId": video_id, "key": self.api_key}

        try:
            response = requests.get(url, params=params)
            response.raise_for_status()
            data = response.json()

            tracks = []
            for item in data.get("items", []):
                track_info = item["snippet"]
                tracks.append(
                    {
                        "id": item["id"],
                        "language": track_info.get("language", ""),
                        "name": track_info.get("name", ""),
                        "is_auto": track_info.get("trackKind") == "ASR",
                    }
                )

            return tracks
        except requests.RequestException as e:
            logger.exception(f"Error listing caption tracks: {str(e)}")
            return []

    def get_caption_content(self, video_id: str, prefer_manual: bool = True) -> dict[str, Any]:
        """Get caption content for a video.

        Args:
            video_id: YouTube video ID
            prefer_manual: Whether to prefer manual captions over auto-generated

        Returns:
            Dict with transcription and metadata
        """
        tracks = self.list_caption_tracks(video_id)

        if not tracks:
            # Try alternative method for fetching captions
            return self._get_captions_alternative(video_id)

        # Find best caption track
        selected_track = None

        # First try to find manual captions if preferred
        if prefer_manual:
            manual_tracks = [t for t in tracks if not t["is_auto"]]
            if manual_tracks:
                selected_track = manual_tracks[0]

        # If no manual track found or not preferred, use any available track
        if not selected_track and tracks:
            selected_track = tracks[0]

        if not selected_track:
            return {"error": "No caption tracks available"}

        # Fetch caption content using track ID
        try:
            # For simplicity, we'll use the alternative method since accessing
            # caption content directly requires OAuth 2.0 authorization
            return self._get_captions_alternative(video_id)
        except Exception as e:
            logger.exception(f"Error fetching caption content: {str(e)}")
            return {"error": f"Failed to fetch caption content: {str(e)}"}

    def _get_captions_alternative(self, video_id: str) -> dict[str, Any]:
        """Alternative method to get captions via transcript API.

        Args:
            video_id: YouTube video ID

        Returns:
            Dict with transcription and metadata
        """
        # This is a workaround since the official API requires OAuth for downloading captions

        try:
            # First get video info for title
            video_info = self.get_video_info(video_id)

            # Try to get transcript using the web API
            # Note: This is a simplified implementation and might not be reliable long-term
            response = requests.get(f"https://www.youtube.com/api/timedtext?lang=en&v={video_id}")

            if response.status_code != 200 or not response.text:
                return {"error": "No captions available"}

            # Parse XML data
            soup = BeautifulSoup(response.text, "xml")
            transcript_parts = []

            for text in soup.find_all("text"):
                # Unescape HTML entities and add to list
                transcript_parts.append(html.unescape(text.string or ""))

            if not transcript_parts:
                return {"error": "Empty caption content"}

            # Join transcript parts with proper spacing
            transcription = " ".join(transcript_parts)

            return {
                "transcription": transcription,
                "title": video_info.get("title", ""),
                "language": video_info.get("language", "en"),
            }

        except Exception as e:
            logger.exception(f"Error in alternative caption fetch: {str(e)}")
            return {"error": f"Failed to fetch captions: {str(e)}"}


def main():
    """Main function to process command line arguments and fetch captions."""
    parser = argparse.ArgumentParser(description="Fetch YouTube video captions")
    parser.add_argument("--video_id", required=True, help="YouTube video ID")
    parser.add_argument("--api_key", required=True, help="YouTube Data API key")

    args = parser.parse_args()

    client = YouTubeCaptionClient(args.api_key)
    result = client.get_caption_content(args.video_id)

    # Output result as JSON to stdout
    sys.stdout.write(json.dumps(result))
    sys.stdout.flush()

    # Return error code if failed
    if "error" in result and result["error"]:
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
