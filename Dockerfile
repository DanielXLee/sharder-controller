# Multi-stage Dockerfile for Kubernetes Shard Controller
# This Dockerfile can build both manager and worker components

# Build stage
FROM golang:1.24-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Set working directory
WORKDIR /workspace

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY cmd/ cmd/
COPY pkg/ pkg/

# Build arguments to determine which component to build
ARG COMPONENT=manager
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_DATE=unknown

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.buildDate=${BUILD_DATE}" \
    -a -installsuffix cgo \
    -o /usr/local/bin/app \
    ./cmd/${COMPONENT}

# Final stage
FROM gcr.io/distroless/static:nonroot

# Copy timezone data
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

# Copy CA certificates
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy the binary
COPY --from=builder /usr/local/bin/app /usr/local/bin/app

# Use non-root user
USER 65534:65534

# Set entrypoint
ENTRYPOINT ["/usr/local/bin/app"]