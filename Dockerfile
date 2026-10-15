# Build
FROM golang:1.22-alpine AS build
WORKDIR /src
COPY . .
RUN go build -trimpath -ldflags="-s -w" -o /out/sonos-radio-agent ./cmd/server
RUN mkdir /out/data && cp config.example.json /out/data/config.json

# Runtime
FROM gcr.io/distroless/static-debian12
COPY --from=build /out/sonos-radio-agent /sonos-radio-agent
# The config lives in a directory writable by nonroot: the web app/API writes
# the config atomically (temp file + rename in the same directory).
# Mount it as a volume (rw), e.g. -v sonos-data:/data or a host directory.
COPY --from=build --chown=nonroot:nonroot /out/data /data
VOLUME /data
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/sonos-radio-agent", "-config", "/data/config.json"]
