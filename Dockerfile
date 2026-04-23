# Build stage
FROM golang:1.26-alpine AS builder

WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy the rest of the application code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o phield main.go

# Final stage
FROM alpine:latest

# Install ca-certificates and openssl
RUN apk --no-cache add ca-certificates openssl

WORKDIR /root/

# Copy the binary from the builder stage
COPY --from=builder /app/phield .

# Generate a self-signed SSL certificate
RUN openssl req -x509 -newkey rsa:4096 -keyout key.pem -out cert.pem -sha256 -days 365 -nodes \
    -subj "/C=US/ST=State/L=City/O=Organization/OU=Unit/CN=localhost"

# Expose the default ports
EXPOSE 8080 8443

# Set default environment variables for SSL
ENV PHIELD_CERT_FILE=/root/cert.pem
ENV PHIELD_KEY_FILE=/root/key.pem
ENV PHIELD_PORT=8443

# Command to run the application
CMD ["./phield"]
