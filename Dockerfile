# Use Go base image with Debian Bullseye for building
FROM golang:1.25.5-bullseye AS builder

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

# Build the application with CGO_ENABLED=1 and static linking flags for a smaller final image
# -s: omit symbol table and debug info
# -w: omit DWARF symbol table
# -extldflags "-static": statically link C libraries (important for Alpine/scratch)
RUN CGO_ENABLED=1 GOOS=linux go build -a -installsuffix cgo -ldflags="-s -w -extldflags \"-static\"" -o releasenojutsu ./cmd/releasenojutsu

# Final stage: Use a minimal Alpine image
FROM alpine:3.22

# Install only runtime dependencies (for sqlite3 and tzdata)
RUN apk add --no-cache ca-certificates sqlite-libs tzdata

# Set working directory
WORKDIR /app

# Create necessary directories
RUN mkdir -p /app/logs /app/database

# Copy the binary from builder
COPY --from=builder /build/releasenojutsu .

# Command to run the application
CMD ["./releasenojutsu"]
