# ── Stage 1: generate protobuf code ─────────────────────────────────────────
FROM golang:1.24.4-alpine AS proto-gen

RUN apk add --no-cache protobuf protobuf-dev

RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.35.2 \
 && go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.5.1

WORKDIR /app
COPY proto/ proto/
COPY go.mod go.sum ./
RUN go mod download

RUN mkdir -p gen/metrics gen/aggregator \
 && protoc \
      --proto_path=proto \
      --go_out=gen       --go_opt=paths=source_relative \
      --go-grpc_out=gen  --go-grpc_opt=paths=source_relative \
      metrics/metrics.proto \
      aggregator/aggregator.proto

# ── Stage 2: build binary ────────────────────────────────────────────────────
FROM golang:1.24.4-alpine AS builder

ARG CMD=collector

WORKDIR /app
COPY --from=proto-gen /app/gen ./gen
COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o /app/service ./cmd/${CMD}

# ── Stage 3: minimal runtime ─────────────────────────────────────────────────
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata
COPY --from=builder /app/service /app/service

ENTRYPOINT ["/app/service"]
