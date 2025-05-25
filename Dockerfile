# Dockerfile
# Build stage
FROM golang:1.24-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o scraper-service cmd/server/main.go

# Final stage
FROM alpine:latest

# Install runtime dependencies for Chrome/Chromium
RUN apk add --no-cache \
    chromium \
    nss \
    freetype \
    freetype-dev \
    harfbuzz \
    ca-certificates \
    ttf-freefont \
    && rm -rf /var/cache/apk/*

# Create non-root user
RUN addgroup -g 1001 -S scraper && \
    adduser -S -D -H -u 1001 -h /app -s /sbin/nologin -G scraper -g scraper scraper

# Set working directory
WORKDIR /app

# Copy binary from builder stage
COPY --from=builder /app/scraper-service .

# Copy any additional files if needed
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Change ownership
RUN chown -R scraper:scraper /app

# Switch to non-root user
USER scraper

# Set Chrome path for rod
ENV CHROME_BIN=/usr/bin/chromium-browser

# Expose port
EXPOSE 8081

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8081/health || exit 1

# Run the service
CMD ["./scraper-service"]