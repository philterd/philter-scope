#!/bin/bash
set -e

# Builds the PhilterScope Docker image for amd64 and arm64. Pushing it is a
# separate, manual step: see push-image.sh.
#
# Each architecture is built and loaded under its own tag, so both are here to
# run and test. push-image.sh pushes those tags and joins them into one
# multi-architecture tag. A multi-architecture image cannot be loaded into the
# local Docker daemon, which is why the per-architecture tags exist at all.
#
# Usage:
#   ./build-image.sh               # build the :latest-<arch> tags
#   ./build-image.sh 1.2.3         # build the :1.2.3-<arch> tags
#   ARCHES=amd64 ./build-image.sh  # build one architecture

VERSION=${1:-latest}
IMAGE=${IMAGE:-philterd/philter-scope}
ARCHES=${ARCHES:-"amd64 arm64"}

# Stamped into the binaries and reported by philterscope-serve at /api/health.
# A named version is used as-is; "latest" falls back to the git description so
# a published image never reports the Dockerfile's "dev" default.
STAMP="${VERSION}"
if [ "$VERSION" = "latest" ]; then
    STAMP=$(git describe --tags --always --dirty 2>/dev/null || echo dev)
fi

# The default builder cannot cross-build, so use a container builder.
docker buildx inspect philterscope-builder > /dev/null 2>&1 ||
    docker buildx create --name philterscope-builder --driver docker-container > /dev/null

for arch in $ARCHES; do
    docker buildx build --builder philterscope-builder \
        --platform "linux/${arch}" --load --build-arg "VERSION=${STAMP}" \
        -t "${IMAGE}:${VERSION}-${arch}" .
done

echo
for arch in $ARCHES; do
    echo "Built ${IMAGE}:${VERSION}-${arch} (version ${STAMP})"
done
echo "Push them with: ./push-image.sh ${VERSION}"
