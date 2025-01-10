# Use Go base image with Debian Bullseye for both building and running
FROM golang:1.23.4-bullseye AS builder

# Install build dependencies required by CGO
RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential gcc libc-dev && \
    rm -rf /var/lib/apt/lists/*

# Set working directory
WORKDIR /build

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application with CGO_ENABLED=1
RUN CGO_ENABLED=1 GOOS=linux go build -a -installsuffix cgo -ldflags="-s -w" -o releasenojutsu .

# Final stage
FROM debian:bullseye-slim

# Install only runtime dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates sqlite3 tzdata && \
    rm -rf /var/lib/apt/lists/*

# Set working directory
WORKDIR /app

# Create necessary directories
RUN mkdir -p /app/logs /app/database

# Copy the binary from builder
COPY --from=builder /build/releasenojutsu .

# Command to run the application
CMD ["./releasenojutsu"]
