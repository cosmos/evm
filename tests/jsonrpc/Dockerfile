# Dockerfile for JSON-RPC Compatibility Test Simulator
FROM golang:1.24-alpine AS builder

# Install dependencies
RUN apk add --no-cache git

# Set working directory
WORKDIR /workspace

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy the entire source code
COPY . .

# Build the simulator
WORKDIR /workspace/tests/jsonrpc/simulator
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o simulator .

# Final stage
FROM alpine:latest

# Install ca-certificates and curl for debugging
RUN apk --no-cache add ca-certificates curl

# Set working directory
WORKDIR /app

# Copy the binary
COPY --from=builder /workspace/tests/jsonrpc/simulator/simulator .

# Copy contract files needed at runtime
COPY --from=builder /workspace/tests/jsonrpc/simulator/contracts ./contracts

# Make it executable
RUN chmod +x simulator

# Default command
CMD ["./simulator"]