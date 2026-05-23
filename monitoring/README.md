# MetaRGB Monitoring Stack

Prometheus and Grafana configuration aligned with [MICROSERVICE_METRICS.md](./MICROSERVICE_METRICS.md).

## Components

| Component | Port | URL |
|-----------|------|-----|
| Prometheus | 9090 | http://localhost:9090 |
| Grafana | 3001 | http://localhost:3001 (admin / admin123) |

## Quick Start

```bash
docker-compose up -d prometheus grafana node-exporter health-check-service kong
```

Verify targets: http://localhost:9090/targets

## Dashboard Organization

Dashboards are provisioned from folder structure (`foldersFromFilesStructure: true`):

| Folder | Dashboard | Purpose |
|--------|-----------|---------|
| **01-overview** | Overview - Golden Signals & Health | SRE golden signals, service health summary |
| **02-services** | Services - Per-Service Metrics | gRPC request rate, latency, errors per service |
| **03-infrastructure** | Infrastructure - Resources | CPU, memory, disk, network (node-exporter) |
| **04-dependencies** | Dependencies - DB, Cache & APIs | MySQL pools, Redis cache, external APIs |
| **05-api-gateway** | API Traffic, Kong Basic | Kong HTTP metrics, routes, upstream health |

## Prometheus Scrape Targets

| Job | Source | Metrics |
|-----|--------|---------|
| `health-check-service` | :8090/metrics | Service health, DB, cache, external APIs |
| `kong` | :8001/metrics | HTTP traffic, latency, errors |
| `node-exporter` | :9100/metrics | Host CPU, memory, disk, network |
| `*-service` | :9090/metrics | Per-service gRPC metrics (when exposed) |

## Recording Rules

Pre-aggregated golden signals in `prometheus/recording_rules.yml`:

- `metargb:http_requests_per_second:5m` — Traffic
- `metargb:http_latency_p50/p95/p99:5m` — Latency
- `metargb:http_error_percentage:5m` — Errors (5xx)
- `metargb:service_availability_percentage` — Availability
- `metargb:node_cpu_usage_percentage` / `metargb:node_memory_usage_percentage` — Saturation

## Alerting Rules

Configured in `prometheus/alerting_rules.yml` per MICROSERVICE_METRICS.md:

### Critical (immediate response)
- Service down (`service_health_status == 0`)
- Error rate > 5% (services or gateway)
- P99 latency > 1s
- CPU > 90% for 5m
- Memory > 95%
- Scrape target down

### Warning (investigation)
- Error rate > 1%
- P95 latency > 500ms
- CPU > 70% for 15m
- Cache hit rate < 80%
- Database connection failures / pool exhaustion
- External API down

### Info (monitoring)
- Service recovered
- Traffic spike (>2x hourly average)
- Monitoring operational

## Configuration Files

```
monitoring/
├── MICROSERVICE_METRICS.md
├── prometheus/
│   ├── prometheus.yml
│   ├── recording_rules.yml
│   └── alerting_rules.yml
└── grafana/
    ├── datasources/prometheus.yml
    ├── provisioning/dashboards/dashboard-provider.yml
    └── dashboards/
        ├── 01-overview/
        ├── 02-services/
        ├── 03-infrastructure/
        ├── 04-dependencies/
        └── 05-api-gateway/
```

## Per-Service Metrics

Services using `metargb/shared/pkg/metrics` expose:

- `metargb_<service>_requests_total{method,status}`
- `metargb_<service>_request_duration_seconds_bucket{method,le}`
- `metargb_<service>_requests_in_flight{method}`
- `metargb_<service>_db_connection_pool{stat}`

Start an HTTP server on port 9090 with `promhttp.Handler()` for Prometheus to scrape.

## Troubleshooting

**Prometheus rules not loading:** Ensure the full `monitoring/prometheus/` directory is mounted (not only `prometheus.yml`).

**Grafana empty panels:** Check http://localhost:9090/targets — all scrape targets should be UP. After config changes, reload Prometheus: `POST http://localhost:9090/-/reload`.

**Grafana cannot reach Prometheus:** Use `http://prometheus:9090` as datasource URL inside Docker.

**Kong golden signals empty (Traffic/Latency/Errors):** The Kong Prometheus plugin must enable optional metrics in `kong/kong.yml`:

```yaml
- name: prometheus
  config:
    status_code_metrics: true
    latency_metrics: true
    bandwidth_metrics: true
    upstream_health_metrics: true
```

Restart Kong after changes. Metrics like `kong_http_requests_total` only appear after traffic flows through the gateway.

**Per-service gRPC metrics empty:** Services must expose `/metrics` on port 9090 via `metargb/shared/pkg/metrics`. Rebuild and restart services after code changes: `docker compose build <service> && docker compose up -d <service>`.
