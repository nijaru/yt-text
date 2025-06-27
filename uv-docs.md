# uv Python Package Manager Reference Guide

This reference guide provides essential information about `uv`, a high-performance Python package and project manager written in Rust. It serves as a faster alternative to tools like pip, pip-tools, pipx, poetry, pyenv, twine, and virtualenv.

## Installation

### macOS and Linux
```bash
curl -LsSf https://astral.sh/uv/install.sh | sh
```

### Windows
```powershell
irm https://astral.sh/uv/install.ps1 | iex
```

### Using pip
```bash
pip install uv
```

## Core Commands

### Project Management

#### `uv init [OPTIONS] [NAME]`
Creates a new Python project with template files.
```bash
# Create a new project in a subdirectory
uv init myproject

# Initialize a project in the current directory
uv init
```

#### `uv add [OPTIONS] <PACKAGE>...`
Adds one or more packages to the project's dependencies.
```bash
# Add a regular dependency
uv add requests

# Add a development dependency
uv add --dev pytest

# Add a dependency to a specific script
uv add --script build sphinx

# Add a package with version constraint
uv add 'requests==2.31.0'

# Add from git repository
uv add git+https://github.com/psf/requests
```

#### `uv remove [OPTIONS] <PACKAGE>...`
Removes one or more packages from the project's dependencies.
```bash
# Remove a regular dependency
uv remove requests

# Remove a development dependency
uv remove --dev pytest

# Remove a dependency from a specific script
uv remove --script build sphinx
```

### Dependency Management

#### `uv lock [OPTIONS]`
Generates or updates the lockfile with the current dependency graph.
```bash
# Generate/update lockfile
uv lock

# Upgrade all dependencies to latest compatible versions
uv lock --upgrade

# Upgrade specific package to latest compatible version
uv lock --upgrade-package requests

# Generate lockfile with specific Python version constraints
uv lock --python-version 3.9
```

#### `uv sync [OPTIONS]`
Synchronizes the project's environment with the lockfile.
```bash
# Sync dependencies from lockfile
uv sync

# Include development dependencies
uv sync --dev

# Perform exact sync (removes extraneous packages)
uv sync --exact
```

#### `uv export [OPTIONS]`
Exports the project's lockfile to an alternate format, like requirements.txt.
```bash
# Export to requirements.txt
uv export --requirements requirements.txt
```

#### `uv tree [OPTIONS]`
Displays the project's dependency tree.
```bash
# Show dependency tree
uv tree

# Show dependency tree with development dependencies
uv tree --dev
```

### Command Execution

#### `uv run [OPTIONS] <COMMAND>...`
Executes a command within the project's environment.
```bash
# Run a Python script
uv run python main.py

# Run a specific script within the environment
uv run --script build

# Run with specific Python version
uv run --python 3.9 python main.py

# Download and run a script from URL
uv run https://example.com/script.py

# Run with environment variables
uv run --env-file .env python main.py
```

### Environment Management

#### `uv venv [OPTIONS]`
Creates a new virtual environment for the project.
```bash
# Create a virtual environment
uv venv

# Create a virtual environment with specific Python version
uv venv --python 3.9

# Specify location for virtual environment
uv venv --path ./custom-venv
```

### Tool Management

#### `uv tool run [OPTIONS] <COMMAND>...` (or `uvx`)
Runs a tool provided by a Python package in an ephemeral environment.
```bash
# Run a tool (using uvx alias)
uvx ruff check src/

# Run a specific version of a tool
uvx 'ruff==0.0.282' check src/
```

#### `uv tool install [OPTIONS] <PACKAGE>...`
Installs a tool provided by a Python package.
```bash
# Install a tool
uv tool install ruff

# Install a specific version
uv tool install 'ruff==0.0.282'

# Install with a specific Python version
uv tool install --python 3.9 black
```

#### `uv tool uninstall [OPTIONS] <PACKAGE>...`
Uninstalls a previously installed tool.
```bash
# Uninstall a tool
uv tool uninstall ruff
```

### Python Version Management

#### `uv python [OPTIONS] <SUBCOMMAND>`
Manages Python versions.
```bash
# Install specific Python version(s)
uv python install 3.10.4

# Install multiple Python versions
uv python install 3.9.7 3.10.4

# List installed Python versions
uv python list

# Uninstall specific Python version(s)
uv python uninstall 3.10.4

# Pin project to specific Python version
uv python pin 3.9.7
```

### Package Building & Publishing

#### `uv build [OPTIONS] [PACKAGE_DIR]`
Builds Python packages into source distributions and wheels.
```bash
# Build a package
uv build

# Build with specific configuration
uv build --sdist --wheel
```

#### `uv publish [OPTIONS] [DIST_DIR]`
Uploads distributions to an index.
```bash
# Publish a package
uv publish dist/

# Publish to a specific index
uv publish --repository testpypi dist/
```

### pip Compatible Interface

#### `uv pip install [OPTIONS] [PACKAGES]...`
Installs packages with a pip-compatible interface.
```bash
# Install packages
uv pip install requests pandas

# Install from requirements file
uv pip install -r requirements.txt
```

#### `uv pip compile [OPTIONS] [REQUIREMENTS_IN]`
Compiles requirements files with a pip-tools compatible interface.
```bash
# Compile requirements
uv pip compile requirements.in

# Generate universal requirements
uv pip compile --universal requirements.in
```

### Cache Management

#### `uv cache [OPTIONS] <COMMAND>`
Manages uv's cache.
```bash
# Clear the cache
uv cache clear

# Show cache directory
uv cache dir
```

### Self Management

#### `uv self update`
Updates the uv executable.

#### `uv version`
Displays uv's version.

## Common Options and Flags

Many commands support these common options:

- `--color`: Control the use of color in output (auto, always, never)
- `--index`: The URL of a package index
- `--find-links`: Additional locations to search for packages
- `--config-setting`: Settings to pass to the PEP 517 build backend
- `--cache-dir`: Custom path to the cache directory
- `--verbose`: Enable verbose output
- `--quiet`: Suppress informational output

## Common Workflows

### Setting up a new project
```bash
# Create a virtual environment with Python 3.10
uv init myproject
cd myproject
uv venv --python 3.10

# Add dependencies
uv add flask sqlalchemy

# Add development dependencies
uv add --dev pytest black

# Generate lockfile
uv lock

# Sync dependencies
uv sync --dev
```

### Updating dependencies
```bash
# Update all dependencies
uv lock --upgrade

# Update specific dependency
uv lock --upgrade-package flask

# Sync updated dependencies
uv sync --dev
```

### Running scripts in the environment
```bash
# Run tests
uv run pytest

# Run specific Python script
uv run python src/main.py
```

### Using Python tools on demand
```bash
# Run a tool without installing
uvx black src/

# Install a tool for repeated use
uv tool install black
```

### Building and publishing packages
```bash
# Build distributions
uv build

# Publish to PyPI
uv publish dist/
```

### Managing Python versions
```bash
# Install Python versions
uv python install 3.10 3.11

# Pin project to specific version
uv python pin 3.10
```
