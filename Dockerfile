# Use Go base image for building (Debian-based).
FROM golang:1.25.5 AS builder

# Install build dependencies required by CGO
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates build-essential gcc libc-dev && \
    rm -rf /var/lib/apt/lists/*

# Set working directory
WORKDIR /build

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application (CGO is required for github.com/mattn/go-sqlite3).
RUN CGO_ENABLED=1 GOOS=linux go build -trimpath -ldflags="-s -w" -o releasenojutsu ./cmd/releasenojutsu

# Prepare runtime directories (distroless has no shell/tools).
RUN mkdir -p /out/logs /out/database && cp /build/releasenojutsu /out/releasenojutsu

# Final stage: Distroless with glibc support for CGO binaries.
FROM gcr.io/distroless/cc-debian13:nonroot

# Set working directory
WORKDIR /app

# Copy CA certs for outbound HTTPS (MangaDex API).
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt

# Copy binary + writable directories
COPY --chown=65532:65532 --from=builder /out/ /app/

# Command to run the application
CMD ["./releasenojutsu"]
