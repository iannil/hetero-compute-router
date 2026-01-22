# Build stage
FROM golang:1.21-alpine AS builder

WORKDIR /workspace

# Install git and other build tools
RUN apk add --no-cache git ca-certificates

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build node-agent binary
ARG TARGETOS=linux
ARG TARGETARCH=amd64
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -a -ldflags="-w -s" -o node-agent ./cmd/node-agent

# Final stage
FROM alpine:latest

LABEL org.opencontainers.image.source="https://github.com/zrs-io/hetero-compute-router"
LABEL org.opencontainers.image.description="HCS Node Agent - Heterogeneous compute resource collector"
LABEL org.opencontainers.image.licenses="Apache-2.0"

RUN apk --no-cache add ca-certificates

WORKDIR /

COPY --from=builder /workspace/node-agent .

ENTRYPOINT ["/node-agent"]
