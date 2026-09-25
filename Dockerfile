# Use a stable Alpine-based Go image for building
FROM golang:1.23-alpine AS builder

# Install UPX and CA certificates
RUN apk update && apk add --no-cache upx ca-certificates git

# Use go modules and don't let go packages call C code
ENV GO111MODULE=on CGO_ENABLED=0
WORKDIR /build
COPY src/ /build/
ARG VERSION
ENV VERSION=${VERSION:-unknown}
RUN GOOS=linux GOARCH=amd64 \
    go build \
    -mod=vendor \
    -ldflags="-s -w -X main.version=${VERSION}" \
    -o StreamStatus ./...

# Compress the binary and verify the output using UPX
RUN upx -v --lzma --best /build/StreamStatus && upx -vt /build/StreamStatus
RUN mkdir /data

# Use a minimal, durable Alpine image for the final runtime
FROM alpine:latest

# Install CA certificates for HTTPS requests
RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder /build/StreamStatus /
WORKDIR /
ENTRYPOINT ["/StreamStatus"]
LABEL org.opencontainers.image.authors='DiscoMouse'
LABEL org.opencontainers.image.description="Twitch Stream Status (infosecstreams-mirror)"
LABEL org.opencontainers.image.licenses='Apache-2.0'
LABEL org.opencontainers.image.source='https://github.com/infosecstreams-mirror/StreamStatus'
LABEL org.opencontainers.image.url='https://infosecstreams.com'
LABEL org.opencontainers.image.vendor='InfoSec Streams'
