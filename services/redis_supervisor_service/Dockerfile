# Dockerfile for redis_supervisor_service

# Builder stage
FROM golang:1.24.0-bullseye AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -v -o redis-supervisor ./cmd/redis-supervisor

# Final stage
FROM debian:bullseye-slim

WORKDIR /app

COPY --from=builder /app/redis-supervisor .

EXPOSE 8080

CMD ["./redis-supervisor"]
