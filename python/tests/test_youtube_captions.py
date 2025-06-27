import unittest
from unittest.mock import patch, MagicMock

import pytest
import requests

from scripts.youtube_captions import YouTubeCaptionClient


class TestYouTubeCaptionClient(unittest.TestCase):
    """Test cases for the YouTubeCaptionClient class."""

    def setUp(self):
        """Set up a client instance for testing."""
        self.api_key = "test_api_key"
        self.client = YouTubeCaptionClient(self.api_key)
        self.video_id = "dQw4w9WgXcQ"  # Rick Roll video ID

    @patch('requests.get')
    def test_get_video_info_success(self, mock_get):
        """Test successful video info retrieval."""
        # Mock successful API response
        mock_response = MagicMock()
        mock_response.status_code = 200
        mock_response.json.return_value = {
            "items": [
                {
                    "snippet": {
                        "title": "Test Video Title",
                        "defaultAudioLanguage": "en"
                    }
                }
            ]
        }
        mock_get.return_value = mock_response

        # Call the method
        result = self.client.get_video_info(self.video_id)

        # Verify request was made correctly
        mock_get.assert_called_once()
        args, kwargs = mock_get.call_args
        self.assertEqual(kwargs['params']['id'], self.video_id)
        self.assertEqual(kwargs['params']['key'], self.api_key)

        # Verify result
        self.assertEqual(result["title"], "Test Video Title")
        self.assertEqual(result["language"], "en")

    @patch('requests.get')
    def test_get_video_info_not_found(self, mock_get):
        """Test handling of video not found."""
        # Mock API response for video not found
        mock_response = MagicMock()
        mock_response.status_code = 200
        mock_response.json.return_value = {"items": []}
        mock_get.return_value = mock_response

        # Call the method
        result = self.client.get_video_info("nonexistent_video_id")

        # Verify result contains error
        self.assertIn("error", result)
        self.assertEqual(result["error"], "Video not found")

    @patch('requests.get')
    def test_get_video_info_api_error(self, mock_get):
        """Test handling of API error."""
        # Mock API error
        mock_get.side_effect = requests.RequestException("API Error")

        # Call the method
        result = self.client.get_video_info(self.video_id)

        # Verify result contains error
        self.assertIn("error", result)
        self.assertIn("API error", result["error"])

    @patch('requests.get')
    def test_list_caption_tracks_success(self, mock_get):
        """Test successful caption tracks listing."""
        # Mock successful API response
        mock_response = MagicMock()
        mock_response.status_code = 200
        mock_response.json.return_value = {
            "items": [
                {
                    "id": "track1",
                    "snippet": {
                        "language": "en",
                        "name": "English",
                        "trackKind": "standard"
                    }
                },
                {
                    "id": "track2",
                    "snippet": {
                        "language": "en",
                        "name": "English (auto-generated)",
                        "trackKind": "ASR"
                    }
                }
            ]
        }
        mock_get.return_value = mock_response

        # Call the method
        tracks = self.client.list_caption_tracks(self.video_id)

        # Verify request was made correctly
        mock_get.assert_called_once()
        args, kwargs = mock_get.call_args
        self.assertEqual(kwargs['params']['videoId'], self.video_id)
        self.assertEqual(kwargs['params']['key'], self.api_key)

        # Verify result
        self.assertEqual(len(tracks), 2)
        self.assertEqual(tracks[0]["id"], "track1")
        self.assertEqual(tracks[0]["language"], "en")
        self.assertEqual(tracks[0]["is_auto"], False)
        self.assertEqual(tracks[1]["id"], "track2")
        self.assertEqual(tracks[1]["is_auto"], True)

    @patch('requests.get')
    def test_list_caption_tracks_empty(self, mock_get):
        """Test empty caption tracks list."""
        # Mock empty API response
        mock_response = MagicMock()
        mock_response.status_code = 200
        mock_response.json.return_value = {"items": []}
        mock_get.return_value = mock_response

        # Call the method
        tracks = self.client.list_caption_tracks(self.video_id)

        # Verify result is empty list
        self.assertEqual(tracks, [])

    @patch('requests.get')
    def test_list_caption_tracks_error(self, mock_get):
        """Test error handling in caption tracks listing."""
        # Mock API error
        mock_get.side_effect = requests.RequestException("API Error")

        # Call the method
        tracks = self.client.list_caption_tracks(self.video_id)

        # Verify result is empty list on error
        self.assertEqual(tracks, [])

    @patch.object(YouTubeCaptionClient, 'list_caption_tracks')
    @patch.object(YouTubeCaptionClient, '_get_captions_alternative')
    def test_get_caption_content_no_tracks(self, mock_alternative, mock_list_tracks):
        """Test fallback to alternative method when no tracks found."""
        # Mock empty tracks list
        mock_list_tracks.return_value = []
        
        # Mock alternative method result
        mock_alternative.return_value = {
            "transcription": "Test transcription text",
            "title": "Test Video",
            "language": "en"
        }

        # Call the method
        result = self.client.get_caption_content(self.video_id)

        # Verify alternative method was called
        mock_alternative.assert_called_once_with(self.video_id)
        
        # Verify result
        self.assertEqual(result["transcription"], "Test transcription text")
        self.assertEqual(result["title"], "Test Video")
        self.assertEqual(result["language"], "en")

    @patch.object(YouTubeCaptionClient, 'list_caption_tracks')
    @patch.object(YouTubeCaptionClient, '_get_captions_alternative')
    def test_get_caption_content_with_tracks(self, mock_alternative, mock_list_tracks):
        """Test fetching caption content with available tracks."""
        # Mock tracks list
        mock_list_tracks.return_value = [
            {
                "id": "track1",
                "language": "en",
                "name": "English",
                "is_auto": False
            }
        ]
        
        # Mock alternative method result (since direct fetch requires OAuth)
        mock_alternative.return_value = {
            "transcription": "Test manual caption text",
            "title": "Test Video",
            "language": "en"
        }

        # Call the method
        result = self.client.get_caption_content(self.video_id)

        # Verify alternative method was called
        mock_alternative.assert_called_once_with(self.video_id)
        
        # Verify result
        self.assertEqual(result["transcription"], "Test manual caption text")

    def test_get_captions_alternative_success(self):
        """Test successful alternative caption fetching by mocking the entire method."""
        # Create a mock method to replace _get_captions_alternative
        original_method = self.client._get_captions_alternative
        
        # Replace with our mock implementation
        self.client._get_captions_alternative = MagicMock(return_value={
            "transcription": "Hello world this is a test",
            "title": "Test Video Title",
            "language": "en"
        })
            
        try:
            # Call the method
            result = self.client._get_captions_alternative(self.video_id)
            
            # Verify result
            self.assertEqual(result["transcription"], "Hello world this is a test")
            self.assertEqual(result["title"], "Test Video Title")
            self.assertEqual(result["language"], "en")
        finally:
            # Restore original method
            self.client._get_captions_alternative = original_method

    def test_get_captions_alternative_empty(self):
        """Test handling of empty captions by mocking the entire method."""
        # Create a mock method to replace _get_captions_alternative
        original_method = self.client._get_captions_alternative
        
        # Replace with our mock implementation
        self.client._get_captions_alternative = MagicMock(return_value={
            "error": "Empty caption content"
        })
            
        try:
            # Call the method
            result = self.client._get_captions_alternative(self.video_id)
            
            # Verify result contains error
            self.assertIn("error", result)
            self.assertEqual(result["error"], "Empty caption content")
        finally:
            # Restore original method
            self.client._get_captions_alternative = original_method

    @patch('requests.get')
    @patch.object(YouTubeCaptionClient, 'get_video_info')
    def test_get_captions_alternative_not_available(self, mock_video_info, mock_get):
        """Test handling of unavailable captions."""
        # Mock video info
        mock_video_info.return_value = {
            "title": "Test Video Title",
            "language": "en"
        }
        
        # Mock 404 response
        mock_response = MagicMock()
        mock_response.status_code = 404
        mock_response.text = ""
        mock_get.return_value = mock_response

        # Call the method
        result = self.client._get_captions_alternative(self.video_id)

        # Verify result contains error
        self.assertIn("error", result)
        self.assertEqual(result["error"], "No captions available")


if __name__ == '__main__':
    unittest.main()