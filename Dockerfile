# Stage 1: Build binary
FROM golang:1.22-alpine AS builder

WORKDIR /app

# Install git and ca-certificates
RUN apk add --no-commits ca-certificates git

# Copy dependency manifests first for cached layer building
COPY go.mod go.sum* ./
RUN go mod download

# Copy source code and migrations
COPY . .

# Build lightweight static binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /app/simtelemetry-hub ./cmd/api

# Stage 2: Final lightweight image
FROM alpine:3.19

WORKDIR /app

RUN apk add --no-commits ca-certificates tzdata postgresql-client

COPY --from=builder /app/simtelemetry-hub /app/simtelemetry-hub
COPY --from=builder /app/migrations /app/migrations

EXPOSE 8080

CMD ["/app/simtelemetry-hub"]
