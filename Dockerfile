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

# Build binaries
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -ldflags="-w -s" -o node-agent ./cmd/node-agent
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -ldflags="-w -s" -o scheduler ./cmd/scheduler

# Final stage - node-agent
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /

COPY --from=builder /workspace/node-agent .

ENTRYPOINT ["/node-agent"]
