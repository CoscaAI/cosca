# HELM DEPLOYMENT — Cosca Enterprise

> **Version**: 1.0.0 | **Status**: active | **Owner**: DevOps Chief | **Last Updated**: 2026-07-27

## Description
Deploy Cosca to Kubernetes using Helm charts. Covers chart structure, values configuration, multi-environment deployment, CI/CD integration with GitHub Actions, and production hardening. Chart location: `deploy/helm/cosca/`.

## Architecture
```
deploy/helm/cosca/
├── Chart.yaml              ← Chart metadata (name, version, appVersion)
├── values.yaml             ← Default values
├── values.dev.yaml         ← Development overrides
├── values.staging.yaml     ← Staging overrides
├── values.prod.yaml        ← Production overrides
└── templates/
    ├── _helpers.tpl        ← Template helpers (labels, names)
    ├── deployment.yaml     ← Go API deployment
    ├── service.yaml        ← Service + ingress
    ├── pvc.yaml            ← Persistent volume for SQLite data
    └── configmap.yaml      ← Configuration
```

## Values Configuration
```yaml
# values.yaml — defaults
replicaCount: 2
image:
  repository: ghcr.io/alflen7/cosca
  tag: "1.3.0"
  pullPolicy: IfNotPresent

service:
  type: ClusterIP
  restPort: 14120
  grpcPort: 14121

ingress:
  enabled: true
  className: nginx
  hosts:
    - host: cosca.example.com
      paths:
        - path: /api
          port: 14120
        - path: /grpc
          port: 14121
  tls:
    - secretName: cosca-tls
      hosts:
        - cosca.example.com

resources:
  requests:
    memory: "128Mi"
    cpu: "100m"
  limits:
    memory: "512Mi"
    cpu: "500m"

persistence:
  enabled: true
  size: 10Gi
  storageClass: standard

probes:
  liveness:
    httpGet:
      path: /health
      port: 14120
    initialDelaySeconds: 10
    periodSeconds: 30
  readiness:
    httpGet:
      path: /health
      port: 14120
    initialDelaySeconds: 5
    periodSeconds: 10

autoscaling:
  enabled: true
  minReplicas: 2
  maxReplicas: 10
  targetCPUUtilizationPercentage: 70

securityContext:
  runAsNonRoot: true
  runAsUser: 1000
  readOnlyRootFilesystem: true
  allowPrivilegeEscalation: false

networkPolicy:
  enabled: true
  ingressNS: ["monitoring", "ingress-nginx"]
```

## Deployment Commands
```bash
# Lint chart
helm lint deploy/helm/cosca/

# Dry-run (validate without applying)
helm install cosca deploy/helm/cosca/ --dry-run --debug

# Install
helm upgrade --install cosca deploy/helm/cosca/ \
  --namespace cosca --create-namespace \
  -f deploy/helm/cosca/values.prod.yaml

# Verify
helm test cosca -n cosca
kubectl get pods -n cosca
kubectl port-forward svc/cosca 14120:14120 -n cosca
curl http://localhost:14120/health

# Rollback
helm rollback cosca -n cosca

# Uninstall
helm uninstall cosca -n cosca
```

## CI/CD Integration (GitHub Actions)
```yaml
# .github/workflows/deploy.yml
name: Deploy to Kubernetes
on:
  push:
    tags: ['v*']
jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      
      - name: Build and push Docker image
        run: |
          docker build -t ghcr.io/${{ github.repository }}:${{ github.ref_name }} .
          docker push ghcr.io/${{ github.repository }}:${{ github.ref_name }}
      
      - name: Deploy to staging
        run: |
          helm upgrade --install cosca-staging deploy/helm/cosca/ \
            --namespace cosca-staging --create-namespace \
            -f deploy/helm/cosca/values.staging.yaml \
            --set image.tag=${{ github.ref_name }} \
            --wait --timeout 5m
      
      - name: Run smoke tests
        run: |
          kubectl wait --for=condition=ready pod -l app=cosca -n cosca-staging --timeout=60s
          curl -f http://cosca-staging:14120/health
      
      - name: Deploy to production
        if: success()
        run: |
          helm upgrade --install cosca-prod deploy/helm/cosca/ \
            --namespace cosca-prod --create-namespace \
            -f deploy/helm/cosca/values.prod.yaml \
            --set image.tag=${{ github.ref_name }} \
            --wait --timeout 10m
```

## Production Hardening Checklist
- [ ] runAsNonRoot: true (no root containers)
- [ ] readOnlyRootFilesystem: true (immutable containers)
- [ ] allowPrivilegeEscalation: false
- [ ] NetworkPolicy restricting ingress to specific namespaces
- [ ] Resource limits set (CPU + memory)
- [ ] Liveness + readiness probes configured
- [ ] HPA enabled (min 2 replicas)
- [ ] TLS via cert-manager (Let's Encrypt)
- [ ] Secrets via External Secrets Operator (not in values.yaml)
- [ ] PodDisruptionBudget (minAvailable: 1)
- [ ] Node affinity for zone distribution
- [ ] Prometheus ServiceMonitor for metrics scraping

## Troubleshooting
```bash
# Check pod status
kubectl describe pod -l app=cosca -n cosca-prod

# Check logs
kubectl logs -l app=cosca -n cosca-prod --tail=100

# Check events
kubectl get events -n cosca-prod --sort-by='.lastTimestamp'

# Debug with shell (if securityContext allows)
kubectl exec -it deploy/cosca -n cosca-prod -- /bin/sh
```
