# Mô Tả Helm Chart và Hướng Dẫn Triển Khai

## 1. Tổng Quan Helm Chart

Helm chart `hubble-guard` là một bộ công cụ hoàn chỉnh để triển khai hệ thống phát hiện anomaly mạng trên Kubernetes, bao gồm:

- **Anomaly Detector**: Ứng dụng phân tích Hubble flows và phát hiện các bất thường
- **API Server**: REST API và WebSocket server cung cấp dữ liệu cho UI
- **UI**: Giao diện web (React) để hiển thị dashboard và flow viewer
- **Prometheus**: Hệ thống thu thập và lưu trữ metrics
- **Grafana**: Dashboard để trực quan hóa dữ liệu và metrics

## 2. Cấu Trúc Helm Chart

### 2.1. Thư Mục và Files

```
helm/hubble-guard/
├── Chart.yaml                    # Metadata của chart
├── values.yaml                   # Giá trị mặc định
├── README.md                     # Tài liệu cơ bản
├── files/                        # Files tĩnh
│   └── hubble-dashboard.json    # Dashboard Grafana
└── templates/                    # Kubernetes manifests templates
    ├── _helpers.tpl              # Template helpers
    ├── namespace.yaml            # Namespace definition
    ├── serviceaccount.yaml       # ServiceAccount cho Anomaly Detector
    ├── anomaly-detector-configmap.yaml    # ConfigMap cho Anomaly Detector
    ├── anomaly-detector-deploy.yaml       # Deployment cho Anomaly Detector
    ├── anomaly-detector-svc.yaml          # Service cho Anomaly Detector
    ├── ui-deploy.yaml                     # Deployment cho UI
    ├── ui-svc.yaml                       # Service cho UI
    ├── ui-ingress.yaml                   # Ingress cho UI (optional)
    ├── prometheus-configmap.yaml          # ConfigMap cho Prometheus
    ├── prometheus-deploy.yaml             # Deployment cho Prometheus
    ├── prometheus-pvc.yaml                # PersistentVolumeClaim cho Prometheus
    ├── prometheus-svc.yaml                # Service cho Prometheus
    ├── grafana-configmap-dashboard.yaml   # ConfigMap chứa Grafana dashboard
    ├── grafana-configmap-provisioning.yaml # ConfigMap cho Grafana provisioning
    ├── grafana-deploy.yaml                # Deployment cho Grafana
    ├── grafana-pvc.yaml                   # PersistentVolumeClaim cho Grafana
    └── grafana-svc.yaml                   # Service cho Grafana
```

### 2.2. Chart Metadata (Chart.yaml)

- **Tên**: `hubble-guard`
- **Phiên bản**: `1.0.0`
- **Loại**: Application chart
- **Mô tả**: Helm chart cho Hubble Guard - Anomaly Detector, Prometheus, và Grafana

### 2.3. Các Thành Phần Chính

#### 2.3.1. Anomaly Detector

**Deployment** (`anomaly-detector-deploy.yaml`):
- Image: `docker.io/ramseytrinh338/hubble-guard:1.0.0`
- Port: 8080 (HTTP metrics endpoint)
- ConfigMap: Chứa file cấu hình `anomaly_detection.yaml`
- ServiceAccount: Được tạo tự động với quyền cần thiết
- Health checks: Liveness và Readiness probes tại `/metrics`
- Init Container: Chờ Prometheus sẵn sàng trước khi khởi động
- Resources: 
  - Requests: 100m CPU, 128Mi memory
  - Limits: 1000m CPU, 512Mi memory

**Service** (`anomaly-detector-svc.yaml`):
- Type: ClusterIP
- Port: 8080
- Selector: Chọn pods của Anomaly Detector
- Được sử dụng bởi UI để kết nối API (hiện tại API Server có thể chạy cùng với detector)

**ConfigMap** (`anomaly-detector-configmap.yaml`):
- Chứa cấu hình từ `values.yaml` được chuyển đổi sang YAML
- Mount tại `/config/anomaly_detection.yaml` trong container

#### 2.3.2. UI (Web Interface)

**Deployment** (`ui-deploy.yaml`):
- Image: `docker.io/ramseytrinh338/hubble-ui:1.0.0`
- Port: 80 (HTTP)
- Environment Variables:
  - `VITE_API_URL`: URL của API Server (trỏ đến Anomaly Detector service)
  - `VITE_WS_URL`: WebSocket URL cho real-time updates
- Resources:
  - Requests: 50m CPU, 64Mi memory
  - Limits: 200m CPU, 256Mi memory
- Health checks: Liveness và Readiness probes tại `/`

**Service** (`ui-svc.yaml`):
- Type: ClusterIP (có thể đổi thành NodePort hoặc LoadBalancer)
- Port: 80
- Selector: Chọn pods của UI

**Ingress** (`ui-ingress.yaml`):
- Optional: Chỉ được tạo khi `ui.ingress.enabled: true`
- Class: Có thể cấu hình (mặc định: nginx)
- Hosts: Có thể cấu hình trong `values.yaml`
- TLS: Hỗ trợ TLS termination

#### 2.3.3. Prometheus

**Deployment** (`prometheus-deploy.yaml`):
- Image: `prom/prometheus:v3.7.3`
- Port: 9090
- ConfigMap: Chứa file `prometheus.yml` với scrape configs
- Storage: PersistentVolumeClaim (10Gi) hoặc emptyDir
- Retention: 15 ngày (có thể cấu hình)
- Scrape interval: 15 giây

**Service** (`prometheus-svc.yaml`):
- Type: ClusterIP
- Port: 9090
- Được sử dụng bởi Grafana và các service khác

**ConfigMap** (`prometheus-configmap.yaml`):
- Chứa cấu hình scrape metrics từ Anomaly Detector
- Tự động cấu hình target: `anomaly-detector:8080`

**PersistentVolumeClaim** (`prometheus-pvc.yaml`):
- Size: 10Gi (có thể cấu hình)
- StorageClass: Có thể chỉ định hoặc dùng default

#### 2.3.4. Grafana

**Deployment** (`grafana-deploy.yaml`):
- Image: `grafana/grafana:11.0.0`
- Port: 3000
- Admin credentials: admin/admin (có thể cấu hình)
- ConfigMaps:
  - Datasource provisioning: Tự động kết nối với Prometheus
  - Dashboard provisioning: Tự động load dashboards
  - Dashboard JSON: Chứa dashboard Hubble

**Service** (`grafana-svc.yaml`):
- Type: ClusterIP
- Port: 3000

**ConfigMaps**:
- `grafana-configmap-provisioning.yaml`: Cấu hình datasource và dashboard provisioning
- `grafana-configmap-dashboard.yaml`: Chứa dashboard JSON

**PersistentVolumeClaim** (`grafana-pvc.yaml`):
- Optional: Mặc định tắt, có thể bật để lưu trữ dữ liệu Grafana

#### 2.3.5. Namespace

**Namespace** (`namespace.yaml`):
- Tên: `hubble-guard`
- Labels: Quản lý bởi Helm

#### 2.3.6. ServiceAccount

**ServiceAccount** (`serviceaccount.yaml`):
- Được tạo cho Anomaly Detector
- Có thể mở rộng với RBAC nếu cần

### 2.4. Template Helpers (_helpers.tpl)

Các helper functions được định nghĩa:
- `hubble-guard.name`: Tên của chart
- `hubble-guard.fullname`: Tên đầy đủ của release
- `hubble-guard.labels`: Common labels
- `hubble-guard.selectorLabels`: Selector labels
- Component-specific labels và selectors
- Service names cho Prometheus và Grafana

## 3. Cấu Hình (values.yaml)

### 3.1. Application Configuration

```yaml
application:
  hubble_server: "hubble-relay.hubble.svc.cluster.local:4245"
  prometheus_export_url: "8080"
  default_namespace: "default"
  auto_start: false
```

### 3.2. Rules Configuration

Chart hỗ trợ nhiều loại rules:
- `traffic_spike`: Phát hiện traffic spike (DDoS)
- `new_destination`: Phát hiện kết nối đến destination mới
- `tcp_reset_surge`: Phát hiện surge TCP resets
- `tcp_drop_surge`: Phát hiện surge TCP drops
- `high_bandwidth`: Phát hiện bandwidth cao bất thường
- `unusual_port_scan`: Phát hiện port scanning
- `block_connection`: Phát hiện blocked connections
- `port_scan`: Phát hiện port scanning attacks
- `namespace_access`: Phát hiện truy cập trái phép namespace
- `suspicious_outbound`: Phát hiện kết nối outbound đáng ngờ

### 3.3. Alerting Configuration

- Log alerts
- Telegram alerts (cần cấu hình bot_token và chat_id)
- Webhook alerts
- Email alerts

### 3.4. UI Configuration

```yaml
ui:
  enabled: true
  replicaCount: 1
  image:
    repository: docker.io/ramseytrinh338/hubble-ui
    tag: "1.0.0"
  service:
    type: ClusterIP
    port: 80
  ingress:
    enabled: false
    className: "nginx"
    hosts:
      - host: hubble-ui.local
        paths:
          - path: /
            pathType: Prefix
  resources:
    limits:
      cpu: 200m
      memory: 256Mi
    requests:
      cpu: 50m
      memory: 64Mi
```

### 3.5. Resources Configuration

Mỗi component có thể cấu hình:
- Replica count
- Image repository và tag
- Resources (CPU, memory)
- Persistence (cho Prometheus và Grafana)
- Node selector, affinity, tolerations

## 4. Đóng Gói Helm Chart

### 4.1. Kiểm Tra Chart

Trước khi đóng gói, kiểm tra chart:

```bash
# Kiểm tra cú pháp và cấu trúc
helm lint ./helm/hubble-guard

# Xem trước các manifests sẽ được tạo
helm template hubble-guard ./helm/hubble-guard

# Test với dry-run
helm install hubble-guard ./helm/hubble-guard --dry-run --debug
```

### 4.2. Đóng Gói Chart

```bash
# Đóng gói chart thành file .tgz
helm package ./helm/hubble-guard

# Kết quả: hubble-guard-1.0.0.tgz
```

### 4.3. Tạo Helm Repository (Tùy chọn)

Nếu muốn tạo local repository:

```bash
# Tạo thư mục charts
mkdir -p charts-repo

# Di chuyển chart đã đóng gói
mv hubble-guard-1.0.0.tgz charts-repo/

# Tạo index
helm repo index charts-repo/

# Thêm repository
helm repo add local-repo file://$(pwd)/charts-repo
```

## 5. Triển Khai Lên Kubernetes Cluster

### 5.1. Yêu Cầu Trước Khi Triển Khai

1. **Kubernetes Cluster**: Phiên bản 1.19+
2. **Helm**: Phiên bản 3.0+
3. **Hubble**: Đã cài đặt và chạy Hubble Relay service
4. **StorageClass**: Nếu sử dụng persistence cho Prometheus/Grafana
5. **kubectl**: Đã cấu hình và kết nối với cluster

### 5.2. Cấu Hình Values

Tạo file `my-values.yaml` để override các giá trị mặc định:

```yaml
# my-values.yaml
application:
  hubble_server: "hubble-relay.hubble.svc.cluster.local:4245"

anomalyDetector:
  image:
    repository: docker.io/ramseytrinh338/hubble-guard
    tag: "latest"
  resources:
    limits:
      cpu: 2000m
      memory: 1Gi

ui:
  enabled: true
  image:
    repository: docker.io/ramseytrinh338/hubble-ui
    tag: "latest"
  ingress:
    enabled: true
    className: "nginx"
    hosts:
      - host: hubble-ui.example.com
        paths:
          - path: /
            pathType: Prefix

prometheus:
  persistence:
    enabled: true
    size: 20Gi
    storageClass: "fast-ssd"

grafana:
  adminPassword: "secure-password-here"
  persistence:
    enabled: true
    size: 5Gi

alerting:
  telegram:
    bot_token: "YOUR_BOT_TOKEN"
    chat_id: "YOUR_CHAT_ID"
    enabled: true
```

### 5.3. Triển Khai Từ Local Chart

**Cách 1: Triển khai trực tiếp từ thư mục chart**

```bash
# Tạo namespace (nếu chưa có)
kubectl create namespace hubble-guard

# Triển khai với values mặc định
helm install hubble-guard ./helm/hubble-guard -n hubble-guard

# Triển khai với custom values
helm install hubble-guard ./helm/hubble-guard \
  -n hubble-guard \
  -f my-values.yaml

# Hoặc sử dụng --create-namespace (khuyến nghị)
helm install hubble-guard ./helm/hubble-guard \
  -n hubble-guard \
  --create-namespace \
  -f my-values.yaml
```

**Cách 2: Triển khai từ chart đã đóng gói**

```bash
# Triển khai từ file .tgz
helm install hubble-guard ./hubble-guard-1.0.0.tgz \
  -n hubble-guard \
  --create-namespace \
  -f my-values.yaml
```

**Cách 3: Triển khai từ Helm repository**

```bash
# Nếu đã thêm vào repository
helm repo update
helm install hubble-guard local-repo/hubble-guard \
  -n hubble-guard \
  --create-namespace \
  -f my-values.yaml
```

### 5.4. Kiểm Tra Triển Khai

```bash
# Kiểm tra status của release
helm status hubble-guard -n hubble-guard

# Xem danh sách pods
kubectl get pods -n hubble-guard

# Xem logs của Anomaly Detector
kubectl logs -n hubble-guard -l app.kubernetes.io/component=anomaly-detector

# Xem logs của UI
kubectl logs -n hubble-guard -l app.kubernetes.io/component=ui

# Xem services
kubectl get svc -n hubble-guard

# Kiểm tra ConfigMaps
kubectl get configmap -n hubble-guard
```

### 5.5. Cập Nhật Triển Khai

```bash
# Upgrade release với values mới
helm upgrade hubble-guard ./helm/hubble-guard \
  -n hubble-guard \
  -f my-values.yaml

# Hoặc từ chart đã đóng gói
helm upgrade hubble-guard ./hubble-guard-1.0.0.tgz \
  -n hubble-guard \
  -f my-values.yaml
```

### 5.6. Rollback

```bash
# Xem lịch sử releases
helm history hubble-guard -n hubble-guard

# Rollback về phiên bản trước
helm rollback hubble-guard -n hubble-guard

# Rollback về phiên bản cụ thể
helm rollback hubble-guard 2 -n hubble-guard
```

### 5.7. Gỡ Cài Đặt

```bash
# Uninstall release
helm uninstall hubble-guard -n hubble-guard

# Xóa namespace (nếu muốn)
kubectl delete namespace hubble-guard
```

## 6. Truy Cập Services

### 6.1. Port Forwarding

```bash
# UI
kubectl port-forward -n hubble-guard svc/hubble-guard-ui 5000:80
# Truy cập: http://localhost:5000

# Prometheus
kubectl port-forward -n hubble-guard svc/hubble-guard-prometheus 9090:9090
# Truy cập: http://localhost:9090

# Grafana
kubectl port-forward -n hubble-guard svc/hubble-guard-grafana 3000:3000
# Truy cập: http://localhost:3000 (admin/admin)

# Anomaly Detector metrics / API Server
kubectl port-forward -n hubble-guard svc/hubble-guard-anomaly-detector 5001:8080
# Truy cập: 
# - Metrics: http://localhost:5001/metrics
# - API: http://localhost:5001/api/v1
```

### 6.2. Service URLs Trong Cluster

- **UI**: `http://hubble-guard-ui.hubble-guard.svc.cluster.local:80`
- **Anomaly Detector / API Server**: `http://hubble-guard-anomaly-detector.hubble-guard.svc.cluster.local:8080`
  - Metrics: `http://hubble-guard-anomaly-detector.hubble-guard.svc.cluster.local:8080/metrics`
  - API: `http://hubble-guard-anomaly-detector.hubble-guard.svc.cluster.local:8080/api/v1`
- **Prometheus**: `http://hubble-guard-prometheus.hubble-guard.svc.cluster.local:9090`
- **Grafana**: `http://hubble-guard-grafana.hubble-guard.svc.cluster.local:3000`

### 6.3. Ingress (Nếu cần)

UI đã có sẵn Ingress template (`ui-ingress.yaml`). Để kích hoạt:

```yaml
# my-values.yaml
ui:
  ingress:
    enabled: true
    className: "nginx"
    hosts:
      - host: hubble-ui.example.com
        paths:
          - path: /
            pathType: Prefix
    tls:
      - secretName: hubble-ui-tls
        hosts:
          - hubble-ui.example.com
```

Có thể thêm Ingress cho Grafana nếu cần:

```yaml
# templates/grafana-ingress.yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: {{ include "hubble-guard.fullname" . }}-grafana-ingress
  namespace: hubble-guard
spec:
  ingressClassName: nginx
  rules:
    - host: grafana.example.com
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: {{ include "hubble-guard.fullname" . }}-grafana
                port:
                  number: 3000
```

## 7. Troubleshooting

### 7.1. Pods Không Khởi Động

```bash
# Kiểm tra events
kubectl describe pod <pod-name> -n hubble-guard

# Kiểm tra logs
kubectl logs <pod-name> -n hubble-guard

# Kiểm tra ConfigMap
kubectl get configmap -n hubble-guard -o yaml
```

### 7.2. Anomaly Detector Không Kết Nối Được Hubble

- Kiểm tra `application.hubble_server` trong values
- Đảm bảo Hubble Relay service đang chạy
- Kiểm tra network policies

### 7.3. Prometheus Không Scrape Được Metrics

- Kiểm tra Prometheus config trong ConfigMap
- Xem targets trong Prometheus UI: Status > Targets
- Kiểm tra service selector

### 7.4. Grafana Không Hiển Thị Dashboard

- Kiểm tra datasource provisioning
- Kiểm tra dashboard ConfigMap
- Xem logs của Grafana pod

## 8. Best Practices

1. **Security**:
   - Thay đổi mật khẩu Grafana mặc định
   - Sử dụng Secrets cho sensitive data (bot tokens, passwords)
   - Cấu hình RBAC cho ServiceAccount

2. **Persistence**:
   - Bật persistence cho Prometheus để lưu trữ metrics
   - Bật persistence cho Grafana nếu cần lưu custom dashboards

3. **Resources**:
   - Điều chỉnh resources dựa trên workload thực tế
   - Sử dụng HPA (Horizontal Pod Autoscaler) nếu cần

4. **Monitoring**:
   - Thiết lập alerts cho các components
   - Monitor resource usage

5. **Backup**:
   - Backup Prometheus data nếu sử dụng persistence
   - Backup Grafana dashboards

## 9. Tích Hợp CI/CD

### 9.1. GitHub Actions Example

```yaml
name: Deploy to Kubernetes

on:
  push:
    branches: [main]

jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v2
      
      - name: Setup Helm
        uses: azure/setup-helm@v1
      
      - name: Deploy to K8s
        run: |
          helm upgrade --install hubble-guard ./helm/hubble-guard \
            -n hubble-guard \
            --create-namespace \
            -f values/production.yaml
        env:
          KUBECONFIG: ${{ secrets.KUBECONFIG }}
```

### 9.2. GitLab CI Example

```yaml
deploy:
  stage: deploy
  image: alpine/helm:latest
  script:
    - helm upgrade --install hubble-guard ./helm/hubble-guard \
        -n hubble-guard \
        --create-namespace \
        -f values/production.yaml
  only:
    - main
```

## 10. Tài Liệu Tham Khảo

- [Helm Documentation](https://helm.sh/docs/)
- [Kubernetes Documentation](https://kubernetes.io/docs/)
- [Prometheus Documentation](https://prometheus.io/docs/)
- [Grafana Documentation](https://grafana.com/docs/)

