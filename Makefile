# Build the phield binary
build:
	go build -o phield main.go

# Run phield locally (requires MongoDB)
run: build
	./phield

# Build the Docker image
docker-build:
	docker compose build

# Start services using Docker Compose
docker-up:
	docker compose up -d

# Stop services using Docker Compose
docker-down:
	docker compose down

# Run tests
test:
	go test ./...

# Clean up built binary
clean:
	rm -f phield

.PHONY: build run docker-build docker-up docker-down test clean
