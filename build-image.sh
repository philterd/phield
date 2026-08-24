#!/bin/bash
set -e

# Builds the Phield Docker image for amd64 and arm64. Pushing it is a separate,
# manual step: see push-image.sh.
#
# Each architecture is built and loaded under its own tag, so both are here to
# run and test. push-image.sh pushes those tags and joins them into one
# multi-architecture tag.

VERSION=${1:-latest}
IMAGE=${IMAGE:-philterd/phield}
ARCHES=${ARCHES:-"amd64 arm64"}

# The default builder cannot cross-build, so use a container builder.
docker buildx inspect phield-builder > /dev/null 2>&1 ||
    docker buildx create --name phield-builder --driver docker-container > /dev/null

# A named version is baked into the binary. "latest" leaves the default in place.
BUILD_ARGS=()
if [ "$VERSION" != "latest" ]; then
    BUILD_ARGS=(--build-arg "VERSION=${VERSION}")
fi

for arch in $ARCHES; do
    docker buildx build --builder phield-builder \
        --platform "linux/${arch}" --load "${BUILD_ARGS[@]}" \
        -t "${IMAGE}:${VERSION}-${arch}" .
done

echo
for arch in $ARCHES; do
    echo "Built ${IMAGE}:${VERSION}-${arch}"
done
echo "Push them with: ./push-image.sh ${VERSION}"
