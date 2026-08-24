#!/bin/bash
set -e

VERSION=${1:-latest}

docker build -t "philterd/phield:${VERSION}" .

echo
echo "Built philterd/phield:${VERSION}"
echo "Push it with: docker push philterd/phield:${VERSION}"
