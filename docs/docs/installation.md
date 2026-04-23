# Installation and Running

## Prerequisites

- Go 1.26+
- MongoDB 8.0+
- Docker and Docker Compose (optional, for containerized deployment)

## Running with Makefile

The project includes a `Makefile` for common tasks:

- `make build`: Build the Phield binary.
- `make run`: Run Phield locally (requires a running MongoDB).
- `make test`: Run all tests.
- `make docker-up`: Start Phield and MongoDB using Docker Compose.
- `make docker-down`: Stop the services.
- `make clean`: Remove the built binary.

## Running with Docker Compose

The easiest way to run Phield along with its MongoDB dependency is using Docker Compose:

```bash
docker compose build
docker compose up
```

Phield will now be running on port 8443 (HTTPS) and listening for ingest requests.

## Running Locally

If you have a MongoDB instance running locally, you can run Phield directly:

```bash
go build -o phield main.go
./phield
```
