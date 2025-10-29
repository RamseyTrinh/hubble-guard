# Deployment Guide - Hubble Anomaly Detector Helm Chart

## Prerequisites

- Kubernetes cluster (1.19+)
- Helm 3.0+
- kubectl configured to access your cluster
- Hubble Relay installed and accessible
- Prometheus server accessible
- Docker registry (for pushing images)

## Step 1: Build Docker Image

```bash
# Build the image
docker build -t your-registry/hubble-anomaly-detector:1.0.0 .

# Push to registry
docker push your-registry/hubble-anomaly-detector:1.0.0
```

## Step 2: Update values.yaml

Edit `helm/hubble-anomaly-detector/values.yaml`:

```yaml
image:
  repository: your-registry/hubble-anomaly-detector
  tag: "1.0.0"

prometheus:
  url: "http://prometheus-server.monitoring.svc.cluster.local:9090"

application:
  hubble_server: "hubble-relay.hubble.svc.cluster.local:4245"

alerting:
  telegram:
    bot_token: "YOUR_BOT_TOKEN"
    chat_id: "YOUR_CHAT_ID"
    enabled: true
```

## Step 3: Deploy with Helm

```bash
# Install chart
helm install hubble-detector ./helm/hubble-anomaly-detector \
  --namespace hubble \
  --create-namespace

# Or with custom values file
helm install hubble-detector ./helm/hubble-anomaly-detector \
  --namespace hubble \
  --create-namespace \
  -f my-values.yaml

# Or with inline values
helm install hubble-detector ./helm/hubble-anomaly-detector \
  --namespace hubble \
  --create-namespace \
  --set image.repository=your-registry/hubble-anomaly-detector \
  --set image.tag=1.0.0 \
  --set prometheus.url=http://prometheus:9090
```

## Step 4: Verify Deployment

```bash
# Check pods
kubectl get pods -n hubble -l app.kubernetes.io/name=hubble-anomaly-detector

# Check logs
kubectl logs -n hubble -l app.kubernetes.io/name=hubble-anomaly-detector

# Check service
kubectl get svc -n hubble -l app.kubernetes.io/name=hubble-anomaly-detector

# Test metrics endpoint
kubectl port-forward -n hubble svc/hubble-detector-hubble-anomaly-detector 8080:8080
curl http://localhost:8080/metrics
```

## Step 5: Configure Prometheus Scraping

If using Prometheus Operator, enable ServiceMonitor:

```yaml
# In values.yaml
serviceMonitor:
  enabled: true
  interval: 15s
  scrapeTimeout: 10s
```

Then upgrade:

```bash
helm upgrade hubble-detector ./helm/hubble-anomaly-detector \
  --namespace hubble \
  --set serviceMonitor.enabled=true
```

## Upgrading

```bash
helm upgrade hubble-detector ./helm/hubble-anomaly-detector \
  --namespace hubble \
  -f my-values.yaml
```

## Uninstalling

```bash
helm uninstall hubble-detector --namespace hubble
```

## Troubleshooting

### Pod not starting
- Check logs: `kubectl logs -n hubble deployment/hubble-detector-hubble-anomaly-detector`
- Check ConfigMap: `kubectl describe configmap -n hubble hubble-detector-hubble-anomaly-detector-config`

### Cannot connect to Hubble Relay
- Verify Hubble Relay service is running: `kubectl get svc -n hubble | grep hubble-relay`
- Check network policies
- Verify service DNS name is correct

### Cannot connect to Prometheus
- Verify Prometheus is accessible from the pod
- Check network policies
- Verify URL format

### Metrics not being scraped
- Check ServiceMonitor is created: `kubectl get servicemonitor -n hubble`
- Verify Prometheus has ServiceMonitor CRD
- Check Prometheus targets

## Advanced Configuration

### Enable Horizontal Pod Autoscaling

```yaml
autoscaling:
  enabled: true
  minReplicas: 1
  maxReplicas: 5
  targetCPUUtilizationPercentage: 80
```

### Enable Ingress

```yaml
ingress:
  enabled: true
  className: nginx
  hosts:
    - host: hubble-detector.example.com
      paths:
        - path: /
          pathType: Prefix
```

### Resource Limits

```yaml
resources:
  limits:
    cpu: 2000m
    memory: 1Gi
  requests:
    cpu: 500m
    memory: 256Mi
```

