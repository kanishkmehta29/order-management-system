FROM golang:1.23-alpine AS builder

WORKDIR /app

# Copy only what's needed for dependency download
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build with optimizations
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /app/bin/order-service ./order-service/main.go

# Final minimal image
FROM alpine:latest

WORKDIR /app
COPY --from=builder /app/bin/order-service .
COPY .env .

# Expose the gRPC port
EXPOSE 50051

CMD ["./order-service"]