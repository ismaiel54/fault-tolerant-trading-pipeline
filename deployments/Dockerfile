# Build stage
FROM golang:1.21-alpine AS builder

WORKDIR /build

# Copy go mod files
COPY go.mod ./
COPY go.sum* ./
RUN go mod download

# Copy source code
COPY . .

# Build targets for each service
FROM builder AS market-ingestor
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/market-ingestor ./cmd/market-ingestor

FROM builder AS stream-processor
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/stream-processor ./cmd/stream-processor

FROM builder AS order-executor
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/order-executor ./cmd/order-executor

FROM builder AS raft-node
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/raft-node ./cmd/raft-node

# Runtime stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates curl

WORKDIR /app

# Copy binary based on target
ARG TARGET
COPY --from=${TARGET} /app/${TARGET} /app/service

EXPOSE 50051 8080

CMD ["/app/service"]

