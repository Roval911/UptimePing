# Multi-stage build for CLI Service
FROM golang:1.24-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Set working directory and module
WORKDIR /app
ENV GO111MODULE=on

# Copy go mod files
COPY go.mod go.sum ./
COPY pkg/go.mod ./pkg/go.mod
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY services/cli-service ./services/cli-service
COPY pkg ./pkg
COPY proto ./proto
# Run go mod tidy to fix dependencies
RUN go mod tidy

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags='-w -s -extldflags "-static"' \
    -a -installsuffix cgo \
    -o cli-service \
    ./services/cli-service/cmd/main.go

# Final stage
FROM alpine:latest

# Install runtime dependencies
RUN apk --no-cache add ca-certificates tzdata wget

# Create non-root user
RUN addgroup -g 1001 -S appgroup && \
    adduser -u 1001 -S appuser -G appgroup

# Set working directory
WORKDIR /app

# Copy binary from builder stage
COPY --from=builder /app/cli-service .

# Copy config files
COPY services/cli-service/config ./config

# Create necessary directories
RUN mkdir -p logs data && \
    chown -R appuser:appgroup /app

# Switch to non-root user
USER appuser

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD ./cli-service --version || exit 1

# Run the application
CMD ["./cli-service"]
