# Build stage
FROM golang:1.25.1-alpine AS builder

WORKDIR /app

# Install build dependencies including SQLite development files
RUN apk add --no-cache git ca-certificates gcc musl-dev

# Download dependencies first (better layer caching)
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application with CGO enabled for SQLite
RUN CGO_ENABLED=1 GOOS=linux go build -ldflags='-w -s' -o /app/group-service ./cmd/api

# Final stage
FROM alpine:3.19

WORKDIR /app

# Install runtime dependencies including SQLite
RUN apk add --no-cache ca-certificates tzdata sqlite

# Create data directory and set permissions
RUN mkdir -p /app/data && \
    chmod -R 777 /app/data

# Create the directory structure for the database
RUN mkdir -p $(dirname /app/data/groups.db) && \
    touch /app/data/groups.db && \
    chmod 666 /app/data/groups.db

# Copy the binary from builder
COPY --from=builder /app/group-service /app/

# Create a non-root user and switch to it
RUN addgroup -S appgroup && \
    adduser -S appuser -G appgroup && \
    chown -R appuser:appgroup /app

# Switch to non-root user
USER appuser

# Expose the port the app runs on
EXPOSE 8003

# Command to run the application
WORKDIR /app
ENTRYPOINT ["/app/group-service"]