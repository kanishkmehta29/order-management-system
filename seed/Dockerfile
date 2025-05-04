FROM golang:1.23-alpine AS builder

WORKDIR /app

# Copy go.mod and go.sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy the rest of the code
COPY . .

# Build the seed binary
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/bin/seed ./seed/

# Create a minimal image for running the application
FROM alpine:latest

WORKDIR /app

# Copy the binary from the builder stage
COPY --from=builder /app/bin/seed .

# Set environment variables if needed
ENV MONGODB_URI=mongodb://mongodb:27017

# Run the seed binary when the container starts
CMD ["/app/seed"]