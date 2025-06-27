# YT-Text Test Suite

This directory contains tests for the yt-text Python components.

## Running Tests

To run the full test suite:

```bash
cd python
python -m pytest
```

To run tests with coverage reports:

```bash
cd python
python -m pytest --cov=scripts --cov-report=term
```

To run a specific test file:

```bash
cd python
python -m pytest tests/test_validate.py
```

## Test Structure

- `test_validate.py`: Tests for the URL validation functionality
- `test_api.py`: Tests for the API module

## Adding Tests

When adding new tests, follow these guidelines:

1. Create files with the naming pattern `test_*.py`
2. Write test classes that inherit from `unittest.TestCase`
3. Write test methods with names starting with `test_`
4. Use descriptive names that explain what functionality is being tested
5. Include docstrings that describe the purpose of each test

## Mocking External Services

Tests should not make actual network requests or interact with external services:

- Use `unittest.mock` to mock external services like YouTube and Whisper
- Use fixture data instead of actual downloads
- Make tests deterministic and repeatable

## Testing Environment Setup

To set up your testing environment:

1. Make sure your Python dependencies are installed:
   ```bash
   cd python
   uv sync
   ```

2. Add test dependencies:
   ```bash
   uv pip install pytest pytest-cov pytest-mock
   ```

3. Known issues:
   - If you get `ModuleNotFoundError` for external packages like `yt_dlp`, ensure they're installed with `uv pip install yt_dlp`
   - If you encounter import errors with local modules, make sure the import paths are correct

## Test Dependencies

Test dependencies are specified in the `[project.optional-dependencies]` section of pyproject.toml under the `dev` group.