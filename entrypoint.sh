#!/bin/sh
set -e

# Generate a self-signed SSL certificate at runtime so each container gets its
# own certificate instead of one baked into the image. An existing certificate
# (for example, a real one mounted into the container) is never overwritten.
if [ -n "$PHIELD_CERT_FILE" ] && [ -n "$PHIELD_KEY_FILE" ]; then
    if [ ! -f "$PHIELD_CERT_FILE" ] || [ ! -f "$PHIELD_KEY_FILE" ]; then
        echo "Generating self-signed SSL certificate at $PHIELD_CERT_FILE"
        mkdir -p "$(dirname "$PHIELD_CERT_FILE")" "$(dirname "$PHIELD_KEY_FILE")"
        openssl req -x509 -quiet -newkey rsa:4096 \
            -keyout "$PHIELD_KEY_FILE" -out "$PHIELD_CERT_FILE" \
            -sha256 -days 365 -nodes \
            -subj "/C=US/ST=State/L=City/O=Organization/OU=Unit/CN=${PHIELD_CERT_CN:-localhost}"
        chmod 600 "$PHIELD_KEY_FILE"
    fi
fi

exec "$@"
