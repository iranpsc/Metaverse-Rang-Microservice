# Kubernetes Manifests

This directory contains Kubernetes manifests for deploying metarang microservices infrastructure.

## Prerequisites

- Kubernetes cluster (1.25+)
- kubectl configured
- Istio installed (optional, for service mesh)
- Sufficient resources (8+ CPU, 16+ GB RAM minimum)

## Quick Start

### 1. Create Namespace

```bash
kubectl apply -f namespace.yaml
```

### 2. Deploy Shared Configuration

```bash
kubectl apply -f shared-config.yaml
```

**Important:** Update secrets in `shared-config.yaml` before deploying to production!

### 3. Deploy MySQL

```bash
kubectl apply -f mysql-statefulset.yaml
```

Wait for MySQL to be ready:
```bash
kubectl wait --for=condition=ready pod -l app=mysql -n metarang --timeout=300s
```

### 4. Initialize Database Schema

```bash
# Copy schema to MySQL pod
kubectl cp ../scripts/schema.sql metarang/mysql-0:/tmp/schema.sql

# Apply schema
kubectl exec -it mysql-0 -n metarang -- mysql -u root -p metarang_db < /tmp/schema.sql
```

### 5. Deploy Redis

```bash
kubectl apply -f redis-deployment.yaml
```

### 6. Verify Infrastructure

```bash
# Check all pods
kubectl get pods -n metarang

# Check services
kubectl get svc -n metarang

# Check PVCs
kubectl get pvc -n metarang
```

## Infrastructure Components

### MySQL StatefulSet
- Single replica (can be scaled or configured for replication)
- 50GB persistent storage
- Custom configuration via ConfigMap
- Health checks (liveness & readiness)

### Redis Deployment
- Single replica for caching and pub/sub
- 10GB persistent storage
- LRU eviction policy
- AOF persistence enabled

### Shared Configuration
- ConfigMap for non-sensitive configuration
- Secrets for passwords and API keys

## Service URLs (within cluster)

- MySQL: `mysql.metarang.svc.cluster.local:3306`
- Redis: `redis.metarang.svc.cluster.local:6379`

## Scaling

### MySQL
For production, consider:
- MySQL cluster (InnoDB Cluster, Percona XtraDB Cluster)
- Read replicas
- Automated backups

### Redis
For production, consider:
- Redis Sentinel for high availability
- Redis Cluster for sharding
- Persistent storage tuning

## Monitoring

After deploying Prometheus:

```bash
# View MySQL metrics
kubectl port-forward svc/mysql-exporter 9104:9104 -n metarang

# View Redis metrics
kubectl port-forward svc/redis-exporter 9121:9121 -n metarang
```

## Backup

### MySQL Backup

```bash
# Manual backup
kubectl exec mysql-0 -n metarang -- mysqldump -u root -p metarang_db > backup-$(date +%Y%m%d).sql
```

### Automated Backups
Use CronJob (see `cronjobs/mysql-backup.yaml`)

## Troubleshooting

### MySQL won't start
```bash
# Check logs
kubectl logs -f mysql-0 -n metarang

# Check PVC
kubectl describe pvc mysql-data-mysql-0 -n metarang
```

### Can't connect to MySQL
```bash
# Test connection
kubectl run mysql-client --rm -it --image=docker.arvancloud.ir/mysql:8.0 -n metarang -- \
  mysql -h mysql.metarang.svc.cluster.local -u metarang -p
```

### Redis issues
```bash
# Check logs
kubectl logs -f deployment/redis -n metarang

# Test connection
kubectl run redis-client --rm -it --image=docker.arvancloud.ir/redis:7-alpine -n metarang -- \
  redis-cli -h redis.metarang.svc.cluster.local ping
```

## Security Considerations

1. **Secrets Management**: Use Kubernetes Secrets or external secret managers (HashiCorp Vault, AWS Secrets Manager)
2. **Network Policies**: Implement network policies to restrict traffic
3. **RBAC**: Configure RBAC for service accounts
4. **Encryption**: Enable encryption at rest for PVCs
5. **TLS**: Use TLS for MySQL connections (configure in `mysql-config`)

## Production Checklist

- [ ] Update all secrets (remove `changeme-*` values)
- [ ] Configure resource limits based on load testing
- [ ] Set up automated backups
- [ ] Enable monitoring and alerting
- [ ] Configure log aggregation
- [ ] Implement network policies
- [ ] Enable Istio mTLS
- [ ] Set up disaster recovery plan
- [ ] Configure HPA for services
- [ ] Review security policies

