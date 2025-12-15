# Snailbus

![Snailbus Logo](snailbus.png)

Backend service to display system information uploaded by snail-core.

## Overview

Snailbus is a lightweight Go webserver built with Gin that provides a REST API for receiving and displaying system information collected by snail-core. It serves as the central hub for aggregating and managing system diagnostics from multiple hosts.

## Quick Start

### Using Docker Compose (Recommended)

```bash
# Build and start the service
docker-compose up -d

# View logs
docker-compose logs -f

# Stop the service
docker-compose down
```

The service will be available at `http://localhost:8080`

### Local Development

```bash
# Install dependencies
go mod download

# Run the server
go run main.go
```

## API Endpoints

### Health Check
```
GET /health
```

Returns:
```json
{
  "status": "ok",
  "service": "snailbus"
}
```

### Root
```
GET /
```

Returns:
```json
{
  "message": "Welcome to Snailbus API",
  "version": "1.0.0"
}
```

## Development

### Prerequisites

- Go 1.24 or later
- Docker and Docker Compose (for containerized deployment)

### Building

```bash
# Build binary
go build -o snailbus main.go

# Run binary
./snailbus
```

### Docker Build

```bash
# Build Docker image
docker build -t snailbus .

# Run container
docker run -p 8080:8080 snailbus
```

## Project Structure

```
snailbus/
├── main.go              # Main application entry point
├── go.mod              # Go module definition
├── go.sum              # Go module checksums
├── Dockerfile          # Docker build configuration
├── docker-compose.yml  # Docker Compose configuration
├── snailbus.png        # Project logo
└── README.md           # This file
```

## Configuration

The server runs on port 8080 by default. To change the port, modify the `r.Run()` call in `main.go`.

### Environment Variables

- `GIN_MODE`: Set to `release` for production (default: `debug`)
  - `debug`: Development mode with detailed logging
  - `release`: Production mode with optimized performance

## Architecture

Snailbus is designed to be the central collection point for system information gathered by snail-core agents running on various Linux hosts. The service receives, processes, and stores diagnostic data for analysis and monitoring.

## License

MIT

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.
