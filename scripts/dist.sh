#!/usr/bin/env bash
# Build release archives for all supported platforms (static binaries,
# CGO off) plus SHA256SUMS.
#
#   scripts/dist.sh [VERSION]      (default: git describe)
#
# Result: dist/hallido_<VERSION>_<os>_<arch>.tar.gz and dist/SHA256SUMS.
# Contents of each archive: binary sonos-radio-agent, config.example.json,
# sonos-agent.service, README.md, LICENSE (if present).
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"; readonly ROOT
cd "$ROOT"

VERSION="${1:-$(git describe --tags --always --dirty 2>/dev/null || echo dev)}"
readonly VERSION
readonly DIST="dist"

# os/arch[/goarm] -- names as usual for uname -m
readonly TARGETS=(
  "linux/amd64"
  "linux/arm64"
  "linux/arm/7"
  "linux/arm/6"
  "darwin/arm64"
  "darwin/amd64"
)

rm -rf "$DIST"
mkdir -p "$DIST"

for t in "${TARGETS[@]}"; do
  IFS=/ read -r goos goarch goarm <<< "$t"
  label="${goos}_${goarch}"
  [[ -n "$goarm" ]] && label="${goos}_armv${goarm}"
  name="hallido_${VERSION}_${label}"
  stage="$DIST/$name"
  mkdir -p "$stage"
  echo ">> $name"
  CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" GOARM="$goarm" \
    go build -trimpath -ldflags="-s -w" -o "$stage/sonos-radio-agent" ./cmd/server
  cp config.example.json "$stage/"
  cp deploy/systemd/sonos-agent.service "$stage/"
  cp README.md "$stage/"
  [[ -f LICENSE ]] && cp LICENSE "$stage/"
  tar -C "$DIST" -czf "$DIST/$name.tar.gz" "$name"
  rm -rf "$stage"
done

(
  cd "$DIST"
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum ./*.tar.gz | sed 's#\./##' > SHA256SUMS
  else
    shasum -a 256 ./*.tar.gz | sed 's#\./##' > SHA256SUMS
  fi
)
echo ">> done: $DIST/"
ls -1 "$DIST"
