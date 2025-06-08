.PHONY: help install dev test lint format clean docker-build docker-run

help:
	@echo "Available commands:"
	@echo "  install       Install dependencies"
	@echo "  dev          Run development server"
	@echo "  test         Run tests with coverage"
	@echo "  lint         Run linting checks"
	@echo "  format       Format code with black"
	@echo "  clean        Clean up generated files"
	@echo "  docker-build Build Docker image"
	@echo "  docker-run   Run Docker container"

install:
	pip install -r requirements.txt

dev:
	uvicorn src.main:app --reload --port 8001

test:
	pytest

lint:
	flake8 src tests
	mypy src
	black --check src tests

format:
	black src tests

clean:
	find . -type d -name "__pycache__" -exec rm -rf {} + 2>/dev/null || true
	find . -type f -name "*.pyc" -delete
	rm -rf .pytest_cache
	rm -rf .coverage
	rm -rf htmlcov
	rm -rf .mypy_cache

docker-build:
	docker build -t intent-processor:latest .

docker-run:
	docker run -d \
		-p 8001:8001 \
		--env-file .env \
		--name intent-processor \
		intent-processor:latest

docker-stop:
	docker stop intent-processor
	docker rm intent-processor

docker-logs:
	docker logs -f intent-processor