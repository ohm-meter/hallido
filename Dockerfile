# syntax=docker/dockerfile:1
#
# Hallido container image (ADR 0021): one static Go binary on distroless,
# no shell or package manager, non-root (UID 65532), meant to run with a
# read-only root filesystem and no capabilities; only /data is writable
# (config.json, backup, logs/). Multi-arch: linux/amd64, linux/arm64,
# linux/arm/v7 -- the Go compiler cross-compiles, no emulation needed.
#
#   docker buildx build --platform linux/amd64,linux/arm64,linux/arm/v7 .
#
# Base images are pinned by digest (supply chain); update them on purpose
# and record the change in CHANGELOG.md.
ARG GO_IMAGE=golang:1.27-alpine@sha256:f92b6ef800e499660581efdabdf25d9d817a9d124eaf900924f0504e7e27e12d
ARG RUNTIME_IMAGE=gcr.io/distroless/static-debian13:nonroot@sha256:e2e927ec666bae08560abb3c55d0659eceabb657f56b6782ab500a9fc7f555e3

FROM --platform=$BUILDPLATFORM ${GO_IMAGE} AS build
ARG TARGETOS
ARG TARGETARCH
ARG TARGETVARIANT
WORKDIR /src
COPY go.mod ./
COPY cmd ./cmd
COPY internal ./internal
# timetzdata: time zones for schedules are compiled into the binary.
RUN CGO_ENABLED=0 GOOS="$TARGETOS" GOARCH="$TARGETARCH" GOARM="${TARGETVARIANT#v}" \
    go build -trimpath -tags timetzdata -ldflags="-s -w -buildid=" -o /out/hallido ./cmd/server \
 && mkdir -p /out/data/logs

FROM ${RUNTIME_IMAGE}
ARG VERSION=dev
LABEL org.opencontainers.image.title="Hallido" \
      org.opencontainers.image.description="Radio scenes, groups and wake-up schedules for Sonos speakers" \
      org.opencontainers.image.source="https://github.com/ohm-meter/hallido" \
      org.opencontainers.image.licenses="MIT" \
      org.opencontainers.image.version="${VERSION}"
COPY --from=build /out/hallido /hallido
# Owned by the runtime user: a new named volume inherits this ownership.
COPY --from=build --chown=65532:65532 /out/data /data
# Container defaults: real speakers, first-run setup in the web app, log file
# in the data volume, port 8790 (8080 is the QNAP admin port).
ENV SRA_DRIVER=sonos \
    SRA_SETUP=1 \
    SRA_LOG_DIR=/data/logs \
    SRA_LISTEN=:8790
VOLUME /data
EXPOSE 8790
USER 65532:65532
HEALTHCHECK --interval=30s --timeout=5s --start-period=15s --retries=3 \
  CMD ["/hallido", "-healthcheck", "-config", "/data/config.json"]
ENTRYPOINT ["/hallido", "-config", "/data/config.json"]
