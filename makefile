.PHONY: all build test clean docker-build docker-run docker-prod docker-stop docker-prod-stop docker-logs docker-prod-logs python-deps

# Variables
BINARY_NAME=yt-text
GO_FILES=$(shell find . -name '*.go' -not -path "./vendor/*")
DOCKER_IMAGE=yt-text
DOCKER_TAG?=latest

all: clean build test

# Go commands
build:
	cd app && CGO_ENABLED=0 go build -v -o $(BINARY_NAME) ./...

test:
	cd app && go test -v -race ./...

clean:
	rm -f app/$(BINARY_NAME)
	find . -type f -name '*.db' -delete
	find . -type d -name '__pycache__' -exec rm -rf {} +
	find . -type f -name '*.pyc' -delete
	find . -type f -name '*.pyo' -delete
	find . -type f -name '*.pyd' -delete
	find . -type d -name '*.egg-info' -exec rm -rf {} +
	find . -type d -name '*.egg' -exec rm -rf {} +
	find . -type d -name '.uv' -exec rm -rf {} +

# Python dependency management
python-deps:
	cd python && uv sync

python-deps-update:
	cd python && uv lock --upgrade
	cd python && uv sync

python-deps-clean:
	rm -rf python/.uv
	rm -rf python/requirements.lock

# Docker commands
docker-build:
	docker build --no-cache -t $(DOCKER_IMAGE):$(DOCKER_TAG) .

docker-prod:
	docker-compose -f docker-compose.prod.yml build
	docker-compose -f docker-compose.prod.yml up

docker-run:
	docker-compose up

docker-stop:
	docker-compose down

docker-prod-stop:
	docker-compose -f docker-compose.prod.yml down

docker-logs:
	docker-compose logs -f

docker-prod-logs:
	docker-compose -f docker-compose.prod.yml logs -f

version:
	@echo $(DOCKER_IMAGE):$(DOCKER_TAG)

help:
	@echo "Available targets:"
	@echo "  build              - Build the Go binary"
	@echo "  test               - Run tests"
	@echo "  clean              - Clean build artifacts"
	@echo "  python-deps        - Install Python dependencies"
	@echo "  python-deps-update - Update Python dependencies"
	@echo "  python-deps-clean  - Clean Python dependencies"
	@echo "  docker-build       - Build Docker image"
	@echo "  docker-run         - Run with Docker Compose (development)"
	@echo "  docker-prod        - Build and run production Docker setup"
	@echo "  docker-stop        - Stop development Docker containers"
	@echo "  docker-prod-stop   - Stop production Docker containers"
	@echo "  docker-logs        - View development Docker logs"
	@echo "  docker-prod-logs   - View production Docker logs"
	@echo "  version           - Show Docker image version"
