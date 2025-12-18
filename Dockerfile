# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Install git (needed for some Go dependencies)
RUN apk add --no-cache git

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code and migrations
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o snailbus .

# Final stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Copy the binary from builder
COPY --from=builder /app/snailbus .

# Copy migrations directory
COPY --from=builder /app/migrations ./migrations

# Copy OpenAPI specification
COPY --from=builder /app/openapi.yaml ./openapi.yaml

# Expose port 8080
EXPOSE 8080

# Run the binary
CMD ["./snailbus"]
