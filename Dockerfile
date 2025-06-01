FROM golang:1.24 AS builder


WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o runner cmd/runner/main.go

# Production stage
FROM debian:bookworm

ENV DEBIAN_FRONTEND=noninteractive
# install necessary packages for the application 
RUN apt-get update && apt-get install -y \
    ca-certificates \
    curl \
    wget \
    unzip \
    git \
    build-essential \
    gcc \
    g++ \
    python3 \
    gdb \
    strace \
    && rm -rf /var/lib/apt/lists/*

RUN wget https://go.dev/dl/go1.24.3.linux-amd64.tar.gz && \
    tar -xzf go1.24.3.linux-amd64.tar.gz -C /usr/local && \
    rm go1.24.3.linux-amd64.tar.gz

ENV PATH=/usr/local/go/bin:$PATH
ENV GOPATH=/go
ENV GOROOT=/usr/local/go

# Create application user
RUN useradd -r -s /bin/false -m appuser
ENV HOME=/home/appuser
ENV GOCACHE=/tmp/go-build-cache
RUN mkdir -p /tmp/go-build-cache && \
    chown -R appuser:appuser /tmp/go-build-cache

# Create necessary directories
RUN mkdir -p /app/temp /app/configs && \
    chown -R appuser:appuser /app

# Copy built application from builder stage
COPY --from=builder /app/runner /app/
COPY --from=builder /app/configs/ /app/configs/

# Set working directory
WORKDIR /app

# Create temp directory for sandbox operations
RUN mkdir -p /tmp/runner_sandbox && \
    chmod 755 /tmp/runner_sandbox

# Set environment variables with defaults
ENV RUNNER_NATS_URL="nats://localhost:4222"
ENV RUNNER_RUNNER_SANDBOXBASEDIR="/tmp/runner_sandbox"
ENV RUNNER_RUNNER_SANDBOXTYPE="direct"
ENV RUNNER_RUNNER_MAXCONCURRENTJOBS="20"
ENV RUNNER_RUNNER_COMPILATIONTIMEOUTSEC="45"
RUN mkdir -p /tmp/runner_sandbox
RUN chmod 777 /tmp/runner_sandbox

# Expose port if needed (though this service typically connects to NATS)
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD pgrep -f runner || exit 1

# Switch to non-root user for security
USER appuser

# Start the application
CMD ["/app/runner"]
