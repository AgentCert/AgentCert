# Multi-stage build for optimal image size
FROM golang:1.21-alpine AS builder

# Set working directory
WORKDIR /app

# Install git (needed for go modules)
RUN apk add --no-cache git

# Copy go module files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY main.go ./

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o k8s-log-tool .

# Final stage - minimal runtime image
FROM alpine:3.18

# Install ca-certificates for HTTPS calls
RUN apk --no-cache add ca-certificates

# Create non-root user for security
RUN adduser -D -s /bin/sh appuser

WORKDIR /root/

# Copy the binary from builder stage
COPY --from=builder /app/k8s-log-tool .

# Change ownership to non-root user
RUN chown appuser:appuser k8s-log-tool

# Switch to non-root user
USER appuser

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Run the binary
CMD ["./k8s-log-tool"]