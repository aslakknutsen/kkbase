# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o /watcher ./cmd/watcher

# Runtime stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates

# Create a non-root user and group
RUN addgroup -g 1001 -S appgroup && \
    adduser -u 1001 -S appuser -G appgroup

# Use /app as working directory (accessible to non-root users)
WORKDIR /app

# Copy the binary from builder and set permissions
COPY --from=builder /watcher .
RUN chmod +x /app/watcher && \
    chown -R appuser:appgroup /app

# OpenShift runs as arbitrary UID, but we set a default user for non-OpenShift environments
USER 1001

EXPOSE 8080

CMD ["./watcher"]

