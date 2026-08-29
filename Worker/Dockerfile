# Build stage
FROM golang:1.23-alpine AS builder

WORKDIR /app

# Copy the module files and download dependencies
COPY go.mod ./
# RUN go mod download

# Copy all the Go source files
COPY *.go ./

# Compiles the code into an executable file named worker
RUN CGO_ENABLED=0 GOOS=linux go build -o worker .

# Final stage
FROM alpine:latest

WORKDIR /app

# Copy the compiled binary from the builder stage
COPY --from=builder /app/worker .

# Run the worker when the container starts
ENTRYPOINT ["./worker"]