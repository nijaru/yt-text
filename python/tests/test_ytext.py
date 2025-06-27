import unittest
import json
import tempfile
from pathlib import Path
from unittest.mock import patch, MagicMock, mock_open

import pytest

from scripts.ytext import (
    format_transcript,
    sanitize_filename,
    generate_short_unique_identifier,
    extract_root_domain,
    get_expected_transcript_filename,
    load_url_mapping,
    save_url_mapping,
    initialize_directories,
    check_transcript_exists,
    determine_transcript_filename,
    save_transcript,
    update_mapping,
    process_single_url,
    process_urls,
    parse_arguments,
    get_urls,
)


class TestYtextUtilities(unittest.TestCase):
    """Test cases for utility functions in ytext.py module."""

    def test_format_transcript(self):
        """Test transcript formatting with various punctuation."""
        # Test with single sentence
        self.assertEqual(
            format_transcript("This is a test."),
            "This is a test.\n"
        )
        
        # Test with multiple sentences
        self.assertEqual(
            format_transcript("This is a test. This is another sentence! And a question?"),
            "This is a test.\nThis is another sentence!\nAnd a question?\n"
        )
        
        # Test with trailing and leading whitespace
        self.assertEqual(
            format_transcript("  This has whitespace. And trailing spaces.  "),
            "This has whitespace.\nAnd trailing spaces."
        )
        
        # Test with existing newlines
        self.assertEqual(
            format_transcript("This has\nnewlines. And periods."),
            "This has\nnewlines.\nAnd periods."
        )

    def test_sanitize_filename(self):
        """Test filename sanitization for various inputs."""
        # Test with invalid characters
        self.assertEqual(
            sanitize_filename("File: with <invalid> chars?"),
            "File_with_invalid_chars"
        )
        
        # Test with very long name (>100 chars)
        long_name = "A" * 120
        self.assertEqual(len(sanitize_filename(long_name)), 100)
        
        # Test with special characters
        self.assertEqual(
            sanitize_filename("File/with\\special:chars*"),
            "Filewithspecialchars"
        )
        
        # Test with spaces
        self.assertEqual(
            sanitize_filename("File with spaces"),
            "File_with_spaces"
        )

    def test_generate_short_unique_identifier(self):
        """Test generation of unique identifiers from URLs."""
        # Test with default length
        id1 = generate_short_unique_identifier("https://www.youtube.com/watch?v=abc123")
        self.assertEqual(len(id1), 8)
        
        # Test with custom length
        id2 = generate_short_unique_identifier("https://www.youtube.com/watch?v=abc123", length=12)
        self.assertEqual(len(id2), 12)
        
        # Test that different URLs give different IDs
        id3 = generate_short_unique_identifier("https://www.youtube.com/watch?v=def456")
        self.assertNotEqual(id1, id3)
        
        # Test that same URL gives same ID
        id4 = generate_short_unique_identifier("https://www.youtube.com/watch?v=abc123")
        self.assertEqual(id1, id4)

    def test_extract_root_domain(self):
        """Test extraction of root domain from various URLs."""
        # Test YouTube URL
        self.assertEqual(
            extract_root_domain("https://www.youtube.com/watch?v=abc123"),
            "youtube"
        )
        
        # Test different domains
        self.assertEqual(extract_root_domain("https://vimeo.com/123456"), "vimeo")
        self.assertEqual(extract_root_domain("https://instagram.com/p/abc123"), "instagram")
        
        # Test with subdomains
        self.assertEqual(extract_root_domain("https://sub.example.com/path"), "example")
        
        # Test with www prefix
        self.assertEqual(extract_root_domain("https://www.example.org"), "example")
        
        # Test malformed URL (should return 'media')
        self.assertEqual(extract_root_domain("not-a-url"), "media")

    def test_get_expected_transcript_filename(self):
        """Test generation of expected transcript filenames."""
        url = "https://www.youtube.com/watch?v=abc123"
        
        # Get the domain and hash that would be generated
        domain = extract_root_domain(url)
        hash_id = generate_short_unique_identifier(url)
        
        # Expected format is "domain_hash.txt"
        expected = f"{domain}_{hash_id}.txt"
        
        # Verify the function output matches our expectation
        self.assertEqual(get_expected_transcript_filename(url), expected)


class TestYtextFilesystem(unittest.TestCase):
    """Test cases for filesystem-related functions in ytext.py."""
    
    def setUp(self):
        """Set up temporary directory for testing."""
        self.temp_dir = tempfile.TemporaryDirectory()
        self.temp_path = Path(self.temp_dir.name)

    def tearDown(self):
        """Clean up temporary directory."""
        self.temp_dir.cleanup()

    @patch('json.load')
    def test_load_url_mapping_existing_file(self, mock_json_load):
        """Test loading URL mapping from existing file."""
        # Setup mock data
        mock_data = {"https://example.com/video1": "example_123.txt"}
        mock_json_load.return_value = mock_data
        
        # Create a mock file
        mapping_file = self.temp_path / "mapping.json"
        with open(mapping_file, "w") as f:
            f.write("{}")  # Just need the file to exist
        
        # Call the function
        result = load_url_mapping(mapping_file)
        
        # Verify result
        self.assertEqual(result, mock_data)
        mock_json_load.assert_called_once()

    def test_load_url_mapping_nonexistent_file(self):
        """Test loading URL mapping from non-existent file."""
        # Non-existent file path
        mapping_file = self.temp_path / "nonexistent.json"
        
        # Call the function
        result = load_url_mapping(mapping_file)
        
        # Should return empty dict for non-existent file
        self.assertEqual(result, {})

    @patch('json.dump')
    def test_save_url_mapping(self, mock_json_dump):
        """Test saving URL mapping to file."""
        # Setup test data
        mapping = {"https://example.com/video1": "example_123.txt"}
        mapping_file = self.temp_path / "mapping.json"
        
        # Call the function
        save_url_mapping(mapping_file, mapping)
        
        # Verify json.dump was called with correct arguments
        mock_json_dump.assert_called_once()
        # First arg should be the mapping dict
        self.assertEqual(mock_json_dump.call_args[0][0], mapping)
        # Should have indent formatting
        self.assertEqual(mock_json_dump.call_args[1]['indent'], 4)

    def test_initialize_directories(self):
        """Test initialization of necessary directories."""
        # Call the function
        audio_dir, transcripts_dir = initialize_directories(self.temp_path)
        
        # Verify directories were created with correct structure
        self.assertEqual(audio_dir, self.temp_path / "ytext_output" / ".audio")
        self.assertEqual(transcripts_dir, self.temp_path / "ytext_output")
        self.assertTrue(audio_dir.exists())
        self.assertTrue(transcripts_dir.exists())

    def test_check_transcript_exists_found_in_mapping(self):
        """Test checking for existing transcript when found in mapping."""
        # Setup test data
        url = "https://example.com/video1"
        mapping = {url: "existing.txt"}
        transcripts_dir = self.temp_path
        
        # Create the transcript file
        transcript_path = transcripts_dir / "existing.txt"
        with open(transcript_path, "w") as f:
            f.write("Existing transcript")
        
        # Call the function
        exists, found_path = check_transcript_exists(url, transcripts_dir, mapping)
        
        # Verify result
        self.assertTrue(exists)
        self.assertEqual(found_path, transcript_path)

    def test_check_transcript_exists_mapping_file_missing(self):
        """Test checking when mapping exists but file is missing."""
        # Setup test data
        url = "https://example.com/video1"
        mapping = {url: "missing.txt"}  # File doesn't exist
        transcripts_dir = self.temp_path
        
        # Call the function
        exists, found_path = check_transcript_exists(url, transcripts_dir, mapping)
        
        # Verify result - should not find the transcript
        self.assertFalse(exists)
        self.assertIsNone(found_path)

    def test_check_transcript_exists_by_expected_name(self):
        """Test finding transcript by its expected name when not in mapping."""
        # Setup test data
        url = "https://example.com/video1"
        mapping = {}  # Empty mapping
        transcripts_dir = self.temp_path
        
        # Create the transcript file with expected name
        expected_name = get_expected_transcript_filename(url)
        transcript_path = transcripts_dir / expected_name
        with open(transcript_path, "w") as f:
            f.write("Existing transcript")
        
        # Call the function
        exists, found_path = check_transcript_exists(url, transcripts_dir, mapping)
        
        # Verify result
        self.assertTrue(exists)
        self.assertEqual(found_path, transcript_path)
        
        # Mapping should be updated
        self.assertEqual(mapping[url], expected_name)

    def test_check_transcript_not_exists(self):
        """Test checking when transcript doesn't exist at all."""
        # Setup test data
        url = "https://example.com/video1"
        mapping = {}
        transcripts_dir = self.temp_path
        
        # Call the function
        exists, found_path = check_transcript_exists(url, transcripts_dir, mapping)
        
        # Verify result
        self.assertFalse(exists)
        self.assertIsNone(found_path)

    def test_determine_transcript_filename_with_title(self):
        """Test determining filename from result with title."""
        # Setup test data
        url = "https://example.com/video1"
        result = {"title": "Test Video Title"}
        
        # Call the function
        filename = determine_transcript_filename(url, result)
        
        # Verify result - should use sanitized title
        self.assertEqual(filename, "Test_Video_Title.txt")

    def test_determine_transcript_filename_without_title(self):
        """Test determining filename when title is missing."""
        # Setup test data
        url = "https://example.com/video1"
        result = {}  # No title
        
        # Get expected name without .txt
        expected_name = get_expected_transcript_filename(url).replace(".txt", "")
        
        # Call the function
        filename = determine_transcript_filename(url, result)
        
        # Verify result - should use expected name based on URL
        self.assertEqual(filename, f"{expected_name}.txt")

    def test_save_transcript(self):
        """Test saving transcript to file."""
        # Setup test data
        transcript_path = self.temp_path / "transcript.txt"
        text = "This is a test transcript.\nWith multiple lines."
        
        # Call the function
        save_transcript(transcript_path, text)
        
        # Verify file was created with correct content
        self.assertTrue(transcript_path.exists())
        with open(transcript_path, "r") as f:
            content = f.read()
            self.assertEqual(content, text)

    def test_update_mapping(self):
        """Test updating URL mapping."""
        # Setup test data
        mapping = {"existing": "existing.txt"}
        url = "https://example.com/video1"
        filename = "video1.txt"
        
        # Call the function
        update_mapping(mapping, url, filename)
        
        # Verify mapping was updated
        self.assertEqual(mapping[url], filename)
        self.assertEqual(mapping["existing"], "existing.txt")  # Original entry preserved


class TestYtextProcessing(unittest.TestCase):
    """Test cases for URL processing functions in ytext.py."""
    
    def setUp(self):
        """Set up temporary directory and mock objects."""
        self.temp_dir = tempfile.TemporaryDirectory()
        self.temp_path = Path(self.temp_dir.name)
        self.transcripts_dir = self.temp_path / "transcripts"
        self.transcripts_dir.mkdir()
        
        # Create a mock transcriber
        self.transcriber = MagicMock()
        
        # Empty mapping for testing
        self.mapping = {}

    def tearDown(self):
        """Clean up temporary directory."""
        self.temp_dir.cleanup()

    def test_process_single_url_existing_transcript(self):
        """Test processing a URL with existing transcript."""
        # Setup
        url = "https://example.com/video1"
        
        # Create existing transcript
        transcript_path = self.transcripts_dir / "existing.txt"
        with open(transcript_path, "w") as f:
            f.write("Existing transcript")
        
        # Mock check_transcript_exists to return the existing transcript
        with patch('scripts.ytext.check_transcript_exists', return_value=(True, transcript_path)):
            # Call the function
            status, message = process_single_url(url, self.transcripts_dir, self.transcriber, self.mapping)
            
            # Verify result
            self.assertEqual(status, "skipped")
            self.assertIn("Transcript exists", message)
            
            # Transcriber should not be called
            self.transcriber.process_url.assert_not_called()

    def test_process_single_url_transcription_error(self):
        """Test processing a URL with transcription error."""
        # Setup
        url = "https://example.com/video1"
        
        # Mock check_transcript_exists to indicate no existing transcript
        with patch('scripts.ytext.check_transcript_exists', return_value=(False, None)):
            # Mock transcriber to return error
            self.transcriber.process_url.return_value = {
                "error": "Transcription failed",
                "text": None
            }
            
            # Call the function
            status, message = process_single_url(url, self.transcripts_dir, self.transcriber, self.mapping)
            
            # Verify result
            self.assertEqual(status, "failed")
            self.assertEqual(message, "Transcription failed")
            
            # Transcriber should be called exactly once
            self.transcriber.process_url.assert_called_once_with(url)

    def test_process_single_url_success(self):
        """Test successful processing of a URL."""
        # Setup
        url = "https://example.com/video1"
        
        # Mock check_transcript_exists to indicate no existing transcript
        with patch('scripts.ytext.check_transcript_exists', return_value=(False, None)):
            # Mock transcriber to return success
            self.transcriber.process_url.return_value = {
                "text": "Successful transcription",
                "title": "Test Video"
            }
            
            # Mock the transcript formatting and saving
            with patch('scripts.ytext.format_transcript', return_value="Formatted transcription"):
                with patch('scripts.ytext.determine_transcript_filename', return_value="test_video.txt"):
                    with patch('scripts.ytext.save_transcript'):
                        with patch('scripts.ytext.update_mapping'):
                            # Call the function
                            status, message = process_single_url(url, self.transcripts_dir, self.transcriber, self.mapping)
                            
                            # Verify result
                            self.assertEqual(status, "success")
                            self.assertIn("Transcript saved", message)
                            
                            # Verify correct flow
                            self.transcriber.process_url.assert_called_once_with(url)

    def test_process_urls(self):
        """Test processing multiple URLs."""
        # Setup
        urls = [
            "https://example.com/video1",  # Will succeed
            "https://example.com/video2",  # Will fail
            "https://example.com/video3",  # Will be skipped
        ]
        
        # Mock process_single_url to return different results for each URL
        def mock_process_side_effect(url, *args, **kwargs):
            if url == urls[0]:
                return ("success", "Success message")
            elif url == urls[1]:
                return ("failed", "Error message")
            else:
                return ("skipped", "Skip message")
        
        with patch('scripts.ytext.process_single_url', side_effect=mock_process_side_effect):
            # Call the function
            summary = process_urls(urls, self.transcripts_dir, self.transcriber, self.mapping)
            
            # Verify summary
            self.assertEqual(summary["total"], 3)
            self.assertEqual(summary["processed"], 2)  # Success + failed
            self.assertEqual(summary["success"], 1)
            self.assertEqual(summary["failed"], 1)
            self.assertEqual(summary["skipped"], 1)
            self.assertEqual(len(summary["failures"]), 1)
            self.assertEqual(summary["failures"][0]["url"], urls[1])
            self.assertEqual(summary["failures"][0]["error"], "Error message")

    @patch('argparse.ArgumentParser.parse_args')
    def test_parse_arguments(self, mock_parse_args):
        """Test command-line argument parsing."""
        # Setup mock arguments
        mock_args = MagicMock()
        mock_args.url = "https://example.com/video1"
        mock_args.urls = ["https://example.com/video2"]
        mock_args.model = "large-v3-turbo"
        mock_args.prompt = "Test prompt"
        mock_args.chunk_length = 120
        mock_parse_args.return_value = mock_args
        
        # Call the function
        args = parse_arguments()
        
        # Verify returned args match mock
        self.assertEqual(args.url, "https://example.com/video1")
        self.assertEqual(args.urls, ["https://example.com/video2"])
        self.assertEqual(args.model, "large-v3-turbo")
        self.assertEqual(args.prompt, "Test prompt")
        self.assertEqual(args.chunk_length, 120)

    def test_get_urls_with_url_arg(self):
        """Test gathering URLs from args with --url argument."""
        # Setup args
        args = MagicMock()
        args.url = "https://example.com/video1"
        args.urls = []
        
        # Call the function
        urls = get_urls(args)
        
        # Verify result
        self.assertEqual(urls, ["https://example.com/video1"])

    def test_get_urls_with_positional_args(self):
        """Test gathering URLs from positional arguments."""
        # Setup args
        args = MagicMock()
        args.url = None
        args.urls = ["https://example.com/video1", "https://example.com/video2,https://example.com/video3"]
        
        # Call the function
        urls = get_urls(args)
        
        # Verify result - should split comma-separated URLs
        self.assertEqual(urls, [
            "https://example.com/video1",
            "https://example.com/video2",
            "https://example.com/video3"
        ])

    def test_get_urls_with_both_arg_types(self):
        """Test gathering URLs from both --url and positional arguments."""
        # Setup args
        args = MagicMock()
        args.url = "https://example.com/video1"
        args.urls = ["https://example.com/video2"]
        
        # Call the function
        urls = get_urls(args)
        
        # Verify result
        self.assertEqual(urls, [
            "https://example.com/video1",
            "https://example.com/video2"
        ])

    def test_get_urls_with_no_urls(self):
        """Test gathering URLs when none are provided."""
        # Setup args
        args = MagicMock()
        args.url = None
        args.urls = []
        
        # Call the function
        urls = get_urls(args)
        
        # Verify result - should be empty list
        self.assertEqual(urls, [])


if __name__ == '__main__':
    unittest.main()