# Hubble Stack Helm Chart

Helm chart để deploy Hubble Anomaly Detector cùng với Prometheus và Grafana.

## Components

Chart này bao gồm:

1. **Anomaly Detector** - Ứng dụng phát hiện anomaly từ Hubble flows
2. **Prometheus** - Thu thập và lưu trữ metrics
3. **Grafana** - Dashboard để visualize metrics

## Cài đặt

Chart này sẽ tự động tạo và deploy vào namespace `hubble-guard`. Có 2 cách deploy:

**Cách 1: Sử dụng --create-namespace (Khuyến nghị)**
```bash
# Helm sẽ tự động tạo namespace nếu chưa có
helm install hubble-guard ./helm/hubble-guard -n hubble-guard --create-namespace

# Với custom values
helm install hubble-guard ./helm/hubble-guard -n hubble-guard --create-namespace -f my-values.yaml
```

**Cách 2: Tạo namespace trước**
```bash
# Tạo namespace trước
kubectl create namespace hubble-guard

# Sau đó deploy
helm install hubble-guard ./helm/hubble-guard -n hubble-guard
```

**Lưu ý:** Chart có template `namespace.yaml` để tạo namespace, nhưng khi dùng `-n` flag, Helm sẽ tạo namespace trước khi apply templates, nên template namespace có thể bị bỏ qua. Cách tốt nhất là dùng `--create-namespace` flag.

## Cấu hình

### Anomaly Detector

Cấu hình chính trong `values.yaml`:

- `application.hubble_server`: Địa chỉ Hubble Relay server
- `application.prometheus_export_url`: Port để export Prometheus metrics
- `rules`: Danh sách các rules để phát hiện anomaly
- `alerting`: Cấu hình alerting (Telegram, webhook, etc.)

### Prometheus

- `prometheus.scrapeInterval`: Khoảng thời gian scrape metrics (mặc định: 15s)
- `prometheus.retention`: Thời gian lưu trữ data (mặc định: 15d)
- `prometheus.persistence.enabled`: Bật/tắt persistent storage

### Grafana

- `grafana.adminUser`: Username admin (mặc định: admin)
- `grafana.adminPassword`: Password admin (mặc định: admin)
- `grafana.persistence.enabled`: Bật/tắt persistent storage

## Truy cập Services

Sau khi deploy vào namespace `hubble-guard`:

- **Anomaly Detector**: `http://<release-name>-anomaly-detector.hubble-guard.svc.cluster.local:8080`
- **Prometheus**: `http://<release-name>-prometheus.hubble-guard.svc.cluster.local:9090`
- **Grafana**: `http://<release-name>-grafana.hubble-guard.svc.cluster.local:3000`

Để truy cập từ bên ngoài cluster, có thể:

1. Sử dụng port-forward:
```bash
kubectl port-forward -n hubble-guard svc/<release-name>-prometheus 9090:9090
kubectl port-forward -n hubble-guard svc/<release-name>-grafana 3000:3000
```

2. Hoặc cấu hình Ingress (cần thêm vào chart)

## Dashboard

Grafana sẽ tự động load dashboard từ ConfigMap. Dashboard mặc định hiển thị:
- Total Hubble Flows
- Flows by Verdict
- Flows by Namespace
- TCP Flags Distribution
- TCP Drops/Connections Over Time
- Portscan metrics
- App CPU/Memory Usage

## Uninstall

```bash
helm uninstall hubble-guard -n hubble-guard
```

## Notes

- Prometheus sẽ tự động scrape metrics từ Anomaly Detector
- Grafana sẽ tự động kết nối với Prometheus qua datasource provisioning
- Dashboard được provisioned tự động từ ConfigMap

