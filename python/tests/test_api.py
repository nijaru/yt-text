import unittest
from unittest.mock import patch, MagicMock, call
import pytest
import json
import sys
from io import StringIO

from scripts.api import main


class TestAPI(unittest.TestCase):
    """Test cases for the API module."""

    @patch('scripts.api.Transcriber')
    @patch('sys.argv', ['api.py', '--url', 'https://www.youtube.com/watch?v=dQw4w9WgXcQ'])
    @patch('sys.stdout')
    def test_successful_transcription(self, mock_stdout, mock_transcriber_class):
        """Test successful transcription with a single URL."""
        # Mock transcriber instance
        mock_transcriber = MagicMock()
        mock_transcriber_class.return_value = mock_transcriber
        
        # Set up mock transcription result
        mock_result = {
            "text": "This is a test transcription",
            "model_name": "large-v3-turbo",
            "duration": 300,
            "error": None,
            "title": "Test Video",
            "url": "https://www.youtube.com/watch?v=dQw4w9WgXcQ",
            "language": "en",
            "language_probability": 0.99,
        }
        mock_transcriber.process_url.return_value = mock_result
        
        # Call the main function
        main()
        
        # Verify the transcriber was called with the correct URL
        mock_transcriber.process_url.assert_called_once_with('https://www.youtube.com/watch?v=dQw4w9WgXcQ')
        
        # Verify the result was written to stdout
        mock_stdout.write.assert_called_once()
        written_data = mock_stdout.write.call_args[0][0]
        output = json.loads(written_data)
        
        # Verify the output structure
        self.assertEqual(output['text'], "This is a test transcription")
        self.assertEqual(output['model_name'], "large-v3-turbo")
        self.assertEqual(output['duration'], 300)
        self.assertEqual(output['language'], "en")
        self.assertEqual(output['title'], "Test Video")

    @patch('scripts.api.Transcriber')
    @patch('sys.argv', ['api.py', '--url', 'https://www.youtube.com/watch?v=invalid'])
    @patch('sys.stdout')
    def test_transcription_error(self, mock_stdout, mock_transcriber_class):
        """Test transcription with an error."""
        # Mock transcriber instance
        mock_transcriber = MagicMock()
        mock_transcriber_class.return_value = mock_transcriber
        
        # Set up mock transcription result with an error
        mock_result = {
            "text": None,
            "model_name": "large-v3-turbo",
            "duration": 0,
            "error": "Video unavailable",
            "title": None,
            "url": "https://www.youtube.com/watch?v=invalid",
            "language": None,
            "language_probability": 0,
        }
        mock_transcriber.process_url.return_value = mock_result
        
        # Call the main function
        main()
        
        # Verify the result was written to stdout
        mock_stdout.write.assert_called_once()
        written_data = mock_stdout.write.call_args[0][0]
        output = json.loads(written_data)
        
        # Verify the error is in the output
        self.assertIsNone(output['text'])
        self.assertEqual(output['error'], "Video unavailable")

    @patch('scripts.api.Transcriber')
    @patch('sys.argv', ['api.py', '--url', 'https://www.youtube.com/watch?v=video1,https://www.youtube.com/watch?v=video2'])
    @patch('sys.stdout')
    def test_multiple_urls(self, mock_stdout, mock_transcriber_class):
        """Test processing multiple URLs."""
        # Mock transcriber instance
        mock_transcriber = MagicMock()
        mock_transcriber_class.return_value = mock_transcriber
        
        # Set up mock transcription results
        mock_results = [
            {
                "text": "First video transcription",
                "model_name": "large-v3-turbo",
                "duration": 300,
                "error": None,
                "title": "Video 1",
                "url": "https://www.youtube.com/watch?v=video1",
                "language": "en",
                "language_probability": 0.99,
            },
            {
                "text": "Second video transcription",
                "model_name": "large-v3-turbo",
                "duration": 400,
                "error": None,
                "title": "Video 2",
                "url": "https://www.youtube.com/watch?v=video2",
                "language": "en",
                "language_probability": 0.98,
            }
        ]
        mock_transcriber.process_url.side_effect = mock_results
        
        # Call the main function
        main()
        
        # Verify the transcriber was called for both URLs
        self.assertEqual(mock_transcriber.process_url.call_count, 2)
        
        # Verify the result was written to stdout
        mock_stdout.write.assert_called_once()
        written_data = mock_stdout.write.call_args[0][0]
        output = json.loads(written_data)
        
        # Verify the output is a list with two items
        self.assertEqual(len(output), 2)
        self.assertEqual(output[0]['text'], "First video transcription")
        self.assertEqual(output[1]['text'], "Second video transcription")

    @patch('scripts.api.Transcriber')
    @patch('sys.argv', ['api.py', '--url', '', '--model', 'large-v3-turbo'])
    @patch('sys.stdout')
    @patch('sys.exit')
    def test_empty_url(self, mock_exit, mock_stdout, mock_transcriber_class):
        """Test behavior when an empty URL is provided."""
        # Call the main function
        main()
        
        # Verify sys.exit was called with status 1
        mock_exit.assert_called_once_with(1)
        
        # Verify error JSON was written to stdout
        mock_stdout.write.assert_called_once()
        written_data = mock_stdout.write.call_args[0][0]
        output = json.loads(written_data)
        
        # Verify the error response
        self.assertIsNone(output['text'])
        self.assertEqual(output['model_name'], 'large-v3-turbo')
        self.assertEqual(output['duration'], 0)
        self.assertIn('No valid URLs provided', output['error'])
        self.assertIsNone(output['url'])

    @patch('scripts.api.Transcriber')
    @patch('sys.argv', ['api.py', '--url', 'https://youtube.com/watch?v=video1,https://youtube.com/watch?v=video2', 
                       '--enable_constraints'])
    @patch('sys.stdout')
    def test_with_constraints_enabled(self, mock_stdout, mock_transcriber_class):
        """Test that only one URL is processed when constraints are enabled."""
        # Mock transcriber instance
        mock_transcriber = MagicMock()
        mock_transcriber_class.return_value = mock_transcriber
        
        # Set up mock transcription result
        mock_result = {
            "text": "Test transcription",
            "model_name": "large-v3-turbo",
            "duration": 300,
            "error": None,
            "title": "Test Video",
            "url": "https://youtube.com/watch?v=video1",
            "language": "en",
            "language_probability": 0.99,
        }
        mock_transcriber.process_url.return_value = mock_result
        
        # Call the main function
        main()
        
        # Verify the transcriber was initialized with constraints
        mock_transcriber_class.assert_called_once_with(
            model_name='large-v3-turbo',
            max_video_duration=4 * 3600,  # 4 hours
            max_file_size=100 * 1024 * 1024,  # 100MB
            chunk_length_seconds=120,
            initial_prompt=None
        )
        
        # Verify only the first URL was processed due to constraints
        mock_transcriber.process_url.assert_called_once_with('https://youtube.com/watch?v=video1')

    @patch('scripts.api.Transcriber')
    @patch('sys.argv', ['api.py', '--url', 'https://youtube.com/watch?v=video1', 
                       '--prompt', 'Technical video'])
    @patch('sys.stdout')
    def test_with_custom_prompt(self, mock_stdout, mock_transcriber_class):
        """Test that custom prompt is passed to transcriber."""
        # Mock transcriber instance
        mock_transcriber = MagicMock()
        mock_transcriber_class.return_value = mock_transcriber
        
        # Set up mock transcription result
        mock_result = {
            "text": "Test transcription",
            "model_name": "large-v3-turbo",
            "duration": 300,
            "error": None,
            "title": "Test Video",
            "url": "https://youtube.com/watch?v=video1",
            "language": "en",
            "language_probability": 0.99,
        }
        mock_transcriber.process_url.return_value = mock_result
        
        # Call the main function
        main()
        
        # Verify the transcriber was initialized with the custom prompt
        mock_transcriber_class.assert_called_once_with(
            model_name='large-v3-turbo',
            max_video_duration=None,
            max_file_size=None,
            chunk_length_seconds=120,
            initial_prompt='Technical video'
        )

    @patch('scripts.api.Transcriber')
    @patch('sys.argv', ['api.py', '--url', 'https://youtube.com/watch?v=video1', 
                       '--chunk_length', '60'])
    @patch('sys.stdout')
    def test_with_custom_chunk_length(self, mock_stdout, mock_transcriber_class):
        """Test that custom chunk length is passed to transcriber."""
        # Mock transcriber instance
        mock_transcriber = MagicMock()
        mock_transcriber_class.return_value = mock_transcriber
        
        # Set up mock transcription result
        mock_result = {
            "text": "Test transcription",
            "model_name": "large-v3-turbo",
            "duration": 300,
            "error": None,
            "title": "Test Video",
            "url": "https://youtube.com/watch?v=video1",
            "language": "en",
            "language_probability": 0.99,
        }
        mock_transcriber.process_url.return_value = mock_result
        
        # Call the main function
        main()
        
        # Verify the transcriber was initialized with the custom chunk length
        mock_transcriber_class.assert_called_once_with(
            model_name='large-v3-turbo',
            max_video_duration=None,
            max_file_size=None,
            chunk_length_seconds=60,  # Custom value
            initial_prompt=None
        )

    @patch('scripts.api.Transcriber')
    @patch('sys.argv', ['api.py', '--url', 'https://youtube.com/watch?v=video1'])
    @patch('sys.stdout')
    def test_empty_transcription_result(self, mock_stdout, mock_transcriber_class):
        """Test handling of empty transcription result."""
        # Mock transcriber instance
        mock_transcriber = MagicMock()
        mock_transcriber_class.return_value = mock_transcriber
        
        # Set up mock transcription with empty text
        mock_result = {
            "text": "",  # Empty text
            "model_name": "large-v3-turbo",
            "duration": 300,
            "error": None,
            "title": "Test Video",
            "url": "https://youtube.com/watch?v=video1",
            "language": "en",
            "language_probability": 0.99,
        }
        mock_transcriber.process_url.return_value = mock_result
        
        # Call the main function
        main()
        
        # Verify the result was written to stdout
        mock_stdout.write.assert_called_once()
        written_data = mock_stdout.write.call_args[0][0]
        output = json.loads(written_data)
        
        # Verify error was added for empty text
        self.assertEqual(output['text'], "")
        self.assertEqual(output['error'], "No transcription text was generated")

    @patch('scripts.api.Transcriber')
    @patch('sys.argv', ['api.py', '--url', 'https://youtube.com/watch?v=video1'])
    @patch('sys.stdout')
    def test_unexpected_exception(self, mock_stdout, mock_transcriber_class):
        """Test handling of unexpected exceptions."""
        # Mock transcriber instance to raise an exception
        mock_transcriber = MagicMock()
        mock_transcriber_class.return_value = mock_transcriber
        mock_transcriber.process_url.side_effect = Exception("Unexpected test error")
        
        # Call the main function
        main()
        
        # Verify the result was written to stdout
        mock_stdout.write.assert_called_once()
        written_data = mock_stdout.write.call_args[0][0]
        output = json.loads(written_data)
        
        # Verify error response
        self.assertIsNone(output['text'])
        self.assertEqual(output['model_name'], 'large-v3-turbo')
        self.assertEqual(output['duration'], 0)
        self.assertIn('Unexpected error', output['error'])
        self.assertIn('Unexpected test error', output['error'])
        self.assertEqual(output['url'], 'https://youtube.com/watch?v=video1')


if __name__ == '__main__':
    unittest.main()