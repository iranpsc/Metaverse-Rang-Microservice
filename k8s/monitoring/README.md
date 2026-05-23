# MetaRGB Monitoring Stack

> **Production Grafana empty but local works?** See **[PRODUCTION.md](./PRODUCTION.md)** for the full fix guide (Istio scrape blocking, Kong plugin, dashboard metric mismatch, image rollout).

## Quick Reference

### Access Dashboards

```bash
# Grafana (Visualization)
kubectl port-forward -n monitoring svc/grafana 3000:3000
# Open: http://localhost:3000
# Login: admin / changeme123!

# Prometheus (Metrics)
kubectl port-forward -n monitoring svc/prometheus 9090:9090
# Open: http://localhost:9090

# Jaeger (Tracing)
kubectl port-forward -n istio-system svc/jaeger-query 16686:16686
# Open: http://localhost:16686

# Kiali (Service Mesh)
kubectl port-forward -n istio-system svc/kiali 20001:20001
# Open: http://localhost:20001
```

### Common PromQL Queries

**Request Rate (RPS) by Service:**
```promql
sum(rate(grpc_server_handled_total{namespace="metargb"}[5m])) by (job)
```

**Error Rate (%) by Service:**
```promql
sum(rate(grpc_server_handled_total{grpc_code!="OK",namespace="metargb"}[5m])) by (job)
/ sum(rate(grpc_server_handled_total{namespace="metargb"}[5m])) by (job) * 100
```

**P95 Latency by Service:**
```promql
histogram_quantile(0.95, 
  sum(rate(grpc_server_handling_seconds_bucket{namespace="metargb"}[5m])) by (job, le)
)
```

**Database Connection Pool Usage:**
```promql
db_connections_in_use{namespace="metargb"} 
/ db_connections_max{namespace="metargb"} * 100
```

**Memory Usage (%):**
```promql
container_memory_working_set_bytes{namespace="metargb"} 
/ container_spec_memory_limit_bytes{namespace="metargb"} * 100
```

### Check Monitoring Health

```bash
# Check all monitoring pods
kubectl get pods -n monitoring
kubectl get pods -n istio-system

# Check Prometheus targets
kubectl port-forward -n monitoring svc/prometheus 9090:9090
# Visit: http://localhost:9090/targets

# Check Grafana data source
kubectl exec -it -n monitoring deploy/grafana -- \
  wget -O- http://prometheus:9090/api/v1/query?query=up
```

### Troubleshooting

**Service not showing metrics:**
```bash
# Check ServiceMonitor
kubectl get servicemonitor -n metargb

# Check service labels
kubectl get svc <service-name> -n metargb --show-labels

# Check Prometheus logs
kubectl logs -n monitoring -l app=prometheus
```

**No traces in Jaeger:**
```bash
# Check Istio tracing config
kubectl get telemetry -n istio-system -o yaml

# Check sidecar logs
kubectl logs <pod-name> -n metargb -c istio-proxy | grep jaeger
```

**Grafana shows no data:**
```bash
# Test Prometheus connection
kubectl exec -it -n monitoring deploy/grafana -- \
  wget -O- http://prometheus:9090/api/v1/query?query=up

# Restart Grafana
kubectl rollout restart deployment grafana -n monitoring
```

### Alerts

View active alerts:
```bash
kubectl port-forward -n monitoring svc/prometheus 9090:9090
# Visit: http://localhost:9090/alerts
```

### Backup

**Backup Grafana Dashboards:**
```bash
kubectl exec -it -n monitoring deploy/grafana -- \
  curl -H "Content-Type: application/json" \
  http://admin:changeme123!@localhost:3000/api/search?type=dash-db | \
  jq -r '.[] | .uri' | \
  xargs -I {} curl -H "Content-Type: application/json" \
  http://admin:changeme123!@localhost:3000/api/dashboards/{} > backup.json
```

**Backup Prometheus Data:**
```bash
# Snapshot current data
kubectl exec -it -n monitoring prometheus-xxxxx -- \
  promtool tsdb create-blocks-from openmetrics /prometheus
```

### Resource Usage

**Current usage:**
```bash
kubectl top pods -n monitoring
kubectl top pods -n istio-system
```

### File Structure

```
monitoring/
├── prometheus/
│   ├── namespace.yaml              # Monitoring namespace
│   ├── prometheus-deployment.yaml  # Prometheus + PVC + RBAC
│   ├── alerting-rules.yaml         # Alert definitions
│   └── service-monitors.yaml       # Scrape configs for services
├── grafana/
│   ├── grafana-deployment.yaml     # Grafana + PVC + Secret
│   └── dashboards-configmap.yaml   # Dashboard JSON definitions
├── jaeger/
│   ├── jaeger-deployment.yaml      # Jaeger all-in-one + PVC
│   └── jaeger-istio-config.yaml    # Istio integration
└── README.md                        # This file
```

### Next Steps

1. **Change Grafana password**:
   ```bash
   kubectl exec -it -n monitoring deploy/grafana -- \
     grafana-cli admin reset-admin-password <new-password>
   ```

2. **Configure alert notifications** (Slack, email)

3. **Set up long-term storage** for Prometheus

4. **Create custom dashboards** for business metrics

5. **Review and tune alert thresholds**

For complete documentation, see `../PHASE6_IMPLEMENTATION.md`

