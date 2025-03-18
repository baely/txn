# Build stage
FROM golang:1.23-alpine AS builder

WORKDIR /app

# Install dependencies
RUN apk add --no-cache git

# Copy and download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o james ./cmd/james

# Final stage
FROM alpine:3.19

WORKDIR /app

# Install CA certificates for HTTPS
RUN apk --no-cache add ca-certificates tzdata

# Copy the binary from the builder stage
COPY --from=builder /app/james .

# Copy any static assets
COPY --from=builder /app/internal/monzo/index.html /app/internal/monzo/
COPY --from=builder /app/internal/monzo/coffee-cup.png /app/internal/monzo/

# Create a non-root user
RUN adduser -D appuser
USER appuser

# Expose port
EXPOSE 8080

# Command to run the application
CMD ["./james"]