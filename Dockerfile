# Use Go base image with Debian Bullseye for both building and running
FROM golang:1.23.4-bullseye

# Install runtime, build dependencies, and libraries required by CGO
RUN apt-get update && apt-get install -y --no-install-recommends \
    git ca-certificates sqlite3 tzdata build-essential gcc libc-dev && \
    rm -rf /var/lib/apt/lists/*

# Set working directory
WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code and .env
COPY . .

# Build the application with CGO_ENABLED=1
RUN CGO_ENABLED=1 GOOS=linux go build -a -installsuffix cgo -ldflags="-s -w" -o releasenojutsu .

# Command to run the application
CMD ["./releasenojutsu"]
