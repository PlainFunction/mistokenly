# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Enable Go modules
ENV GO111MODULE=on

# Copy go.mod and go.sum first for better caching
COPY ../../go.mod ../../go.sum ./
RUN go mod download

# Copy source code
COPY ../../ .

# Build with optimizations: strip debug info and symbols
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o api ./cmd/api

# Final stage: Use alpine for user management and health checks
FROM alpine:3.22.2

# Install curl for health checks
RUN apk --no-cache add curl

# Create non-root user
RUN addgroup -g 1001 -S appgroup && \
    adduser -u 1001 -S appuser -G appgroup

# Copy CA certificates (needed for HTTPS requests, e.g., to external APIs)
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy the binary
COPY --from=builder /app/api /api

# Change ownership to non-root user
RUN chown appuser:appgroup /api

# Switch to non-root user
USER appuser

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
  CMD curl -f http://localhost:8080/health || exit 1

# Run the binary
CMD ["/api"]