# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Install protobuf compiler and go plugins (only if needed)
RUN apk add --no-cache protobuf-dev
RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
RUN go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Generate protobuf code if needed
RUN mkdir -p proto/audit
RUN if [ -f proto/audit_service.proto ]; then \
    protoc --go_out=. --go_opt=paths=source_relative \
    --go-grpc_out=. --go-grpc_opt=paths=source_relative \
    proto/audit_service.proto; \
    fi

# Build with optimizations: strip debug info and symbols
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o audit-service ./cmd/audit

# Final stage: Use alpine for user management and health checks
FROM alpine:3.22.2

# Install curl and grpc-health-probe for health checks
RUN apk --no-cache add curl && \
    wget -qO /usr/local/bin/grpc-health-probe https://github.com/grpc-ecosystem/grpc-health-probe/releases/download/v0.4.12/grpc_health_probe-linux-amd64 && \
    chmod +x /usr/local/bin/grpc-health-probe

# Create non-root user
RUN addgroup -g 1001 -S appgroup && \
    adduser -u 1001 -S appuser -G appgroup

# Copy CA certificates (needed for DB/HTTPS connections)
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy the binary
COPY --from=builder /app/audit-service /audit-service

# Copy migrations directory (if the service needs it at runtime)
COPY --from=builder /app/migrations /migrations

# Change ownership to non-root user
RUN chown -R appuser:appgroup /audit-service /migrations

# Switch to non-root user
USER appuser

# Expose port
EXPOSE 9081

# Set environment (optional, can be overridden)
ENV AUDIT_SERVICE_PORT=9081
ENV ENVIRONMENT=production

# Health check (gRPC)
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
  CMD grpc-health-probe -addr=localhost:9081 || exit 1

# Run the binary
CMD ["/audit-service"]