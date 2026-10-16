#!/usr/bin/env bash
# Render the NAS project file from deploy/nas/compose.template.yaml.
#
#   scripts/render-compose.sh <image@sha256:digest> <version> > compose.yaml
#
# The release workflow calls this with the digest of the image it has just
# built and signed, so the project file pins exactly that build (ADR 0021).
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
IMAGE="${1:-}"
VERSION="${2:-}"

if [[ ! "$IMAGE" =~ ^[a-z0-9./_-]+@sha256:[0-9a-f]{64}$ ]]; then
  echo "usage: $0 <registry/name@sha256:<64 hex>> <version>" >&2
  exit 2
fi
if [[ ! "$VERSION" =~ ^v?[0-9A-Za-z.+-]+$ ]]; then
  echo "invalid version: $VERSION" >&2
  exit 2
fi

sed -e "s#__IMAGE__#${IMAGE}#g" -e "s#__VERSION__#${VERSION}#g" \
  "$ROOT/deploy/nas/compose.template.yaml"
