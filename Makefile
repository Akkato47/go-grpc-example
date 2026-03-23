.PHONY: proto-gen proto-gen-docker build-collector build-aggregator build-generator \
        run-collector run-aggregator run-generator \
        docker-up docker-down test

MODULE := github.com/Akkato47/go-grpc-example

# ── Protobuf generation (requires local protoc + plugins) ─────────────────
proto-gen:
	@mkdir -p gen/metrics gen/aggregator
	protoc \
		--proto_path=proto \
		--go_out=gen       --go_opt=paths=source_relative \
		--go-grpc_out=gen  --go-grpc_opt=paths=source_relative \
		metrics/metrics.proto \
		aggregator/aggregator.proto

# ── Protobuf generation via Docker (no local tooling required) ────────────
proto-gen-docker:
	docker run --rm \
		-v "$(CURDIR)":/workspace \
		-w /workspace \
		golang:1.24.4-alpine \
		sh -c "apk add --no-cache protobuf protobuf-dev && \
		       go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.35.2 && \
		       go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.5.1 && \
		       mkdir -p gen/metrics gen/aggregator && \
		       protoc \
		         --proto_path=proto \
		         --go_out=gen       --go_opt=paths=source_relative \
		         --go-grpc_out=gen  --go-grpc_opt=paths=source_relative \
		         metrics/metrics.proto \
		         aggregator/aggregator.proto"

# ── Local build ────────────────────────────────────────────────────────────
build-collector:
	go build -o bin/collector ./cmd/collector

build-aggregator:
	go build -o bin/aggregator ./cmd/aggregator

build-generator:
	go build -o bin/generator ./cmd/generator

build: build-collector build-aggregator build-generator

# ── Local run (run aggregator first, then collector, then generator) ───────
run-aggregator:
	go run ./cmd/aggregator

run-collector:
	go run ./cmd/collector

run-generator:
	go run ./cmd/generator

# ── Docker ─────────────────────────────────────────────────────────────────
docker-up:
	docker compose up --build -d

docker-down:
	docker compose down

docker-logs:
	docker compose logs -f

# ── Tests ──────────────────────────────────────────────────────────────────
test:
	go test ./...

test-v:
	go test -v ./...
