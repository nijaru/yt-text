# CI/CD Pipeline Documentation

This document describes the Continuous Integration and Continuous Deployment (CI/CD) setup for the yt-text project.

## Overview

The CI/CD pipeline consists of two workflows:

1. **CI (Continuous Integration)** - Runs on every push to the main branch and pull requests.
2. **CD (Continuous Deployment)** - Runs after a successful CI run on the main branch.

## CI Workflow (`ci.yml`)

The CI workflow ensures code quality and security by running the following jobs:

### Go Jobs
- **Go Lint**: Uses golangci-lint to check Go code for issues
- **Go Tests**: Builds and tests Go code with race detection

### Python Jobs
- **Python Lint**: Uses Ruff and Mypy for linting and type checking
- **Python Tests**: Runs pytest for Python tests

### Security Jobs
- **Security Scan**: Uses gosec for Go and Bandit for Python code scanning
- **Dependency Review**: Checks for security issues in dependencies (on PRs only)

### Docker Jobs
- **Docker Build and Scan**: Builds the production Docker image and scans it for vulnerabilities using Trivy

## CD Workflow (`cd.yml`)

The CD workflow automatically deploys the application to Fly.io:

- Triggered after a successful CI workflow run on the main branch
- Uses the Fly.io CLI to deploy the application
- Requires a `FLY_API_TOKEN` secret to be set in the repository

## Setup Instructions

### Required GitHub Secrets

To enable deployments to Fly.io, you need to set up the following secret in your GitHub repository:

1. Go to your repository settings → Secrets and variables → Actions
2. Add a new secret:
   - Name: `FLY_API_TOKEN`
   - Value: Your Fly.io API token (get it by running `flyctl auth token` on your local machine)

### Manual Deployment

You can manually trigger the deployment workflow by:

1. Going to the Actions tab in your GitHub repository
2. Selecting the "CD - Deploy to Fly.io" workflow
3. Clicking "Run workflow" and selecting the branch to deploy from

## Workflow Details

### CI Workflow Triggers
- Push to main branch
- Pull requests to main branch
- Manual trigger (workflow_dispatch)

### CD Workflow Triggers
- Successful completion of CI workflow on main branch
- Manual trigger (workflow_dispatch)

## Troubleshooting

If the deployment fails:

1. Check the workflow logs in the GitHub Actions tab
2. Verify that your Fly.io API token is correctly set up
3. Ensure the application passes all tests in the CI workflow
4. Check that the Fly.io configuration in `docker/fly/fly.toml` is valid