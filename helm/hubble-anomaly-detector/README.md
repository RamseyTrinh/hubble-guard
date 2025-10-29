# Hubble Anomaly Detector Helm Chart

This Helm chart deploys the Hubble Anomaly Detector application on a Kubernetes cluster.

## Prerequisites

- Kubernetes 1.19+
- Helm 3.0+
- Hubble Relay service running in the cluster
- Prometheus server accessible from the cluster

## Installation

1. Add your Helm repository (if applicable):
```bash
helm repo add hubble-detector <your-repo-url>
helm repo update
```

2. Install the chart with default values:
```bash
helm install hubble-detector hubble-anomaly-detector/hubble-anomaly-detector
```

3. Install with custom values:
```bash
helm install hubble-detector hubble-anomaly-detector/hubble-anomaly-detector \
  --set prometheus.url=http://prometheus:9090 \
  --set application.hubble_server=hubble-relay:4245 \
  --set alerting.telegram.bot_token="YOUR_BOT_TOKEN" \
  --set alerting.telegram.chat_id="YOUR_CHAT_ID" \
  --set alerting.telegram.enabled=true
```

## Configuration

### Required Configuration

- `prometheus.url`: URL to your Prometheus server
- `application.hubble_server`: Hubble Relay service endpoint

### Optional Configuration

- `alerting.telegram`: Configure Telegram notifications
- `rules`: Enable/disable and configure detection rules
- `resources`: Set CPU and memory limits/requests

### Example values.yaml

See `values.yaml` for all available configuration options.

## Uninstallation

```bash
helm uninstall hubble-detector
```

## ServiceMonitor Support

If you're using Prometheus Operator, you can enable ServiceMonitor:

```yaml
serviceMonitor:
  enabled: true
  interval: 15s
  scrapeTimeout: 10s
```

## Ingress Support

To enable Ingress for accessing metrics:

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

## Building Docker Image

Before deploying, you need to build the Docker image:

```bash
docker build -t hubble-anomaly-detector:1.0.0 .
```

Or use the provided Dockerfile in the repository root.

## Notes

- The application exposes metrics on port 8080 at `/metrics`
- Configuration is stored in a ConfigMap
- Ensure Hubble Relay is accessible at the configured endpoint
- Make sure Prometheus can scrape metrics from the service

