# Build Stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
# CGO_ENABLED=0 for static binary
RUN CGO_ENABLED=0 GOOS=linux go build -o xandeum-backend .

# Run Stage
FROM alpine:latest

WORKDIR /root/

# Install CA certificates for HTTPS calls (pRPC, GeoIP)
RUN apk --no-cache add ca-certificates

# Copy binary from builder
COPY --from=builder /app/xandeum-backend .

# Expose API port
EXPOSE 8080

# Run
CMD ["./xandeum-backend"]
