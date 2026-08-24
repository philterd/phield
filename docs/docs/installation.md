# Installation and Running

## Prerequisites

- Go 1.26+
- MongoDB 8.0+
- Docker and Docker Compose (optional, for containerized deployment)

## Running with Makefile

The project includes a `Makefile` for common tasks:

- `make build`: Build the Phield binary.
- `make run`: Run Phield locally (requires a running MongoDB).
- `make test`: Run all tests. The MongoDB storage tests need a MongoDB instance at `mongodb://localhost:27017`, or at `PHIELD_TEST_MONGO_URI` if set. They skip when no instance is reachable.
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

The API is unauthenticated unless `PHIELD_API_KEY` is set. Set it in `docker-compose.yaml` to require an API key on every request. See [Authentication](api.md#authentication).

## Running Locally

If you have a MongoDB instance running locally, you can run Phield directly:

```bash
go build -o phield main.go
./phield
```

### Simulating Data

Once Phield is running, you can test it by sending simulated data. A script is provided for this purpose:

```bash
./simulate_data.sh
```

If the instance requires an API key, set `PHIELD_API_KEY` in the script's environment and it is sent with each request.

To check a running instance end to end, `smoke-test.sh` ingests data, reads it back, and verifies the results, exiting non-zero if anything fails. Set `PHIELD_URL` and, if the instance requires one, `PHIELD_API_KEY`.

```bash
PHIELD_URL=https://localhost:8443 ./smoke-test.sh
```

This script will send a series of baseline data points followed by a sudden spike to demonstrate trend detection and alerting. See the [API Usage](api.md#simulating-data) page for more configuration options.
