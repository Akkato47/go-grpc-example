# go-grpc-example

Три сервиса на Go, демонстрирующих работу с gRPC и observability-стеком.

## Сервисы

**Generator** — генерирует фейковые метрики (CPU, memory, HTTP, disk и др.) и отправляет их батчами в Collector через bidirectional streaming. Параллельно подписывается на live-обновления от Aggregator (server-side streaming) и периодически запрашивает статистику (unary RPC).

**Collector** — принимает метрики от Generator, хранит их в памяти и асинхронно форвардит в Aggregator через client-side streaming.

**Aggregator** — принимает метрики от Collector, отдаёт статистику (min/max/avg/p95/p99) по запросу и стримит live-обновления подписчикам.

## gRPC

```
generator  ──[bidir stream]──►  collector  ──[client stream]──►  aggregator
generator  ◄──────────────────────────────────[server stream]──  aggregator (WatchMetrics)
generator  ────────────────────────────────────────[unary]────►  aggregator (GetSummary)
```

На каждом сервисе — unary и stream interceptors для логирования и Prometheus-метрик.

## Observability

| Сервис     | gRPC  | Prometheus |
| ---------- | ----- | ---------- |
| collector  | 50051 | 9091       |
| aggregator | 50052 | 9092       |
| Prometheus | —     | 9090       |
| Grafana    | —     | 3000       |

Метрики: `grpc_requests_total`, `grpc_request_duration_seconds`, `grpc_active_streams`, `metrics_received_total`, `aggregations_computed_total`, `aggregator_active_watchers`.

## Запуск

```bash
make proto-gen
go mod tidy
make docker-up
```

Grafana: http://localhost:3000 — дашборд **gRPC Services Overview** добавляется автоматически.
