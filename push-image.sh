#!/bin/bash
set -e

VERSION=${1:-latest}

docker push "philterd/phield:${VERSION}"

echo
echo "Pushed philterd/phield:${VERSION}"
