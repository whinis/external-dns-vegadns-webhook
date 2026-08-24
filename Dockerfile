#Stage 1: Build stage
FROM --platform=$BUILDPLATFORM golang:1.24 AS builder

ARG TARGETOS
ARG TARGETARCH

WORKDIR /app

# Download Go modules
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the application code
COPY . ./

# Build the Go binary with static linking
RUN GOOS=${TARGETOS} GOARCH=${TARGETARCH}  CGO_ENABLED=0 go build -a -o /app/bin/external-dns-vegadns-webhook ./cmd/webhook/main.go

# Stage 2: Runtime stage
FROM debian:bullseye-slim

RUN groupadd --gid 65532 appgroup && \
    useradd --uid 65532 --gid appgroup --shell /bin/bash --create-home appuser

WORKDIR /app


# Install CA certificates
RUN apt-get update && apt-get install -y ca-certificates && \
    apt-get clean && rm -rf /var/lib/apt/lists/*

# Copy the statically linked binary from the builder stage
COPY --chown=appuser:appgroup --from=builder /app/bin/external-dns-vegadns-webhook /app/bin/external-dns-vegadns-webhook

USER appuser

# Set the binary as the entry point
CMD ["/app/bin/external-dns-vegadns-webhook"]
