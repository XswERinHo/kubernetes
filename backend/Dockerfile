# Build stage
FROM golang:1.23-alpine AS builder

WORKDIR /app

# Force Go to use a compatible version or ignore the version mismatch
ENV GOTOOLCHAIN=auto

COPY go.mod go.sum ./
# Edit go.mod to match the container's Go version if needed, or just rely on GOTOOLCHAIN
RUN sed -i 's/go 1.25.2/go 1.23/' go.mod && go mod download

COPY . .
RUN go build -o backend ./cmd/server

# Final stage
FROM alpine:latest

WORKDIR /app

# Install ca-certificates for HTTPS
RUN apk --no-cache add ca-certificates

COPY --from=builder /app/backend .
# Create data directory for alerts.json
RUN mkdir -p data

EXPOSE 8080

# Set environment variable to tell backend it's running in cluster
ENV IN_CLUSTER=true

CMD ["./backend"]
