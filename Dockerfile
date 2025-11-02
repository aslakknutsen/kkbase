# Build stage
FROM registry.access.redhat.com/ubi9/go-toolset:1.24 AS builder

WORKDIR /opt/app-root/src

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -buildvcs=false -o watcher ./cmd/watcher

# Runtime stage
FROM registry.access.redhat.com/ubi9/ubi-minimal:latest

# UBI minimal already includes CA certificates and is OpenShift-ready
# Set working directory
WORKDIR /app

# Copy the binary from builder and set permissions for arbitrary UID compatibility
COPY --from=builder --chown=1001:0 --chmod=775 /opt/app-root/src/watcher .

# Support running as arbitrary UID (OpenShift requirement)
# The binary and directory are owned by root group (0) with group write permissions
RUN chgrp -R 0 /app && \
    chmod -R g=u /app

# Default to non-root user (OpenShift will override with arbitrary UID)
USER 1001

EXPOSE 8080

CMD ["./watcher"]

