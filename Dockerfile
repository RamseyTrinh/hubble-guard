# Build stage
FROM golang:1.21-alpine AS builder

WORKDIR /build

# Install build dependencies
RUN apk add --no-cache git make

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o hubble-anomaly-detector ./cmd/hubble-detector

# Final stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/hubble-anomaly-detector /app/hubble-anomaly-detector

# Copy config file (optional, can be overridden by ConfigMap)
COPY --from=builder /build/configs/anomaly_detection.yaml /app/configs/anomaly_detection.yaml

# Create non-root user
RUN addgroup -g 1000 appuser && \
    adduser -D -u 1000 -G appuser appuser && \
    chown -R appuser:appuser /app

USER appuser

EXPOSE 8080

ENTRYPOINT ["/app/hubble-anomaly-detector"]

