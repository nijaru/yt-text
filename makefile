.PHONY: all build test clean docker-build docker-run lint security-scan

# Variables
BINARY_NAME=yt-text
GO_FILES=$(shell find . -name '*.go' -not -path "./vendor/*")
DOCKER_IMAGE=yt-text
DOCKER_TAG=latest

all: clean build test

# Go commands
build:
	cd app && CGO_ENABLED=0 go build -v -o $(BINARY_NAME) ./...

test:
	cd app && go test -v -race -coverprofile=coverage.txt -covermode=atomic ./...

coverage: test
	cd app && go tool cover -html=coverage.txt

lint:
	cd app && golangci-lint run
	cd python && ruff check .

clean:
	rm -f app/$(BINARY_NAME)
	rm -f app/coverage.txt
	find . -type f -name '*.db' -delete
	find . -type d -name '__pycache__' -exec rm -rf {} +
	find . -type f -name '*.pyc' -delete
	find . -type f -name '*.pyo' -delete
	find . -type f -name '*.pyd' -delete
	find . -type f -name '.coverage' -delete
	find . -type d -name '*.egg-info' -exec rm -rf {} +
	find . -type d -name '*.egg' -exec rm -rf {} +
	find . -type d -name '.pytest_cache' -exec rm -rf {} +
	find . -type d -name '.ruff_cache' -exec rm -rf {} +

# Docker commands
docker-build:
	docker build --no-cache -t $(DOCKER_IMAGE):$(DOCKER_TAG) .

docker-run:
	docker-compose up

docker-stop:
	docker-compose down

docker-logs:
	docker-compose logs -f

# Security
security-scan:
	trivy image $(DOCKER_IMAGE):$(DOCKER_TAG)
	gosec ./...

# Development setup
dev-setup:
	go install golang.org/x/tools/cmd/godoc@latest
	go install github.com/golangci/golint/cmd/golangci-lint@latest
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $$(go env GOPATH)/bin
	pip install ruff

# Helper commands
format:
	cd app && go fmt ./...
	cd python && ruff format .

version:
	@echo $(DOCKER_IMAGE):$(DOCKER_TAG)

help:
	@echo "Available targets:"
	@echo "  build          - Build the Go binary"
	@echo "  test           - Run tests"
	@echo "  coverage       - Generate test coverage report"
	@echo "  clean          - Clean build artifacts"
	@echo "  docker-build   - Build Docker image"
	@echo "  docker-run     - Run with Docker Compose"
	@echo "  docker-stop    - Stop Docker Compose services"
	@echo "  docker-logs    - View Docker Compose logs"
	@echo "  lint           - Run linters"
	@echo "  security-scan  - Run security scanners"
	@echo "  dev-setup     - Install development tools"
	@echo "  format        - Format code"
	@echo "  version       - Show Docker image version"
