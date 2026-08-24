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

# The entrypoint generates a self-signed SSL certificate on container start so
# that each container has its own certificate rather than one shared by every
# image pull.
COPY entrypoint.sh /usr/local/bin/entrypoint.sh
RUN chmod +x /usr/local/bin/entrypoint.sh

# Expose the default ports
EXPOSE 8080 8443

# Set default environment variables for SSL
ENV PHIELD_CERT_FILE=/root/cert.pem
ENV PHIELD_KEY_FILE=/root/key.pem
ENV PHIELD_PORT=8443

# Command to run the application
ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
CMD ["./phield"]
