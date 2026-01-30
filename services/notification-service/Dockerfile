# Multi-stage build for Notification Service
FROM golang:1.24-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Set working directory
WORKDIR /app
ENV GO111MODULE=on

# Create temporary go.work file with only needed modules and correct Go version
RUN echo "go 1.24.0" > go.work && \
    echo "" >> go.work && \
    echo "use (" >> go.work && \
    echo "  ./pkg" >> go.work && \
    echo "  ./proto" >> go.work && \
    echo "  ./services/notification-service" >> go.work && \
    echo ")" >> go.work

# Copy go mod files for required modules
COPY go.mod go.sum ./
COPY pkg/go.mod pkg/go.sum ./pkg/
COPY proto/go.mod proto/go.sum ./proto/
COPY services/notification-service/go.mod services/notification-service/go.sum ./services/notification-service/

# Sync workspace
RUN go work sync

# Download dependencies
RUN go mod download

# Copy source code
COPY services/notification-service ./services/notification-service
COPY pkg ./pkg
COPY proto ./proto

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags='-w -s -extldflags "-static"' \
    -a -installsuffix cgo \
    -o notification-service \
    ./services/notification-service/cmd/main.go

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
COPY --from=builder /app/notification-service .

# Copy config files
COPY services/notification-service/config ./config

# Create necessary directories
RUN mkdir -p logs data && \
    chown -R appuser:appgroup /app

# Switch to non-root user
USER appuser

# Expose ports
EXPOSE 50057

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:50057/health || exit 1

# Run the application
CMD ["./notification-service"]