FROM golang:1.23-alpine AS builder

WORKDIR /app

# Copy only what's needed for dependency download
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build with optimizations
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /app/bin/payment-service ./payment-service/main.go

# Final minimal image
FROM alpine:latest

WORKDIR /app
COPY --from=builder /app/bin/payment-service .
COPY .env .

CMD ["./payment-service"]