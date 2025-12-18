# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Install git (needed for some Go dependencies)
RUN apk add --no-cache git

# Install swag for generating OpenAPI docs
# Ensure GOPATH is set and add it to PATH
ENV GOPATH=/go
ENV PATH=$PATH:$GOPATH/bin
RUN go install github.com/swaggo/swag/cmd/swag@latest

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code and migrations
COPY . .

# Generate OpenAPI specification from code annotations
RUN swag init -g main.go -o docs --parseDependency --parseInternal

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

# Copy generated OpenAPI specification files
COPY --from=builder /app/docs ./docs

# Expose port 8080
EXPOSE 8080

# Run the binary
CMD ["./snailbus"]
