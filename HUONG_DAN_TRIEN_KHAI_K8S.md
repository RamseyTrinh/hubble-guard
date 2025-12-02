# Hướng Dẫn Triển Khai Hubble Guard trên Cụm Kubernetes Mới

Hướng dẫn này sẽ giúp bạn triển khai Hubble Guard (Anomaly Detector + Prometheus + Grafana) lên một cụm Kubernetes mới sử dụng Helm.

## 📋 Yêu Cầu Trước Khi Triển Khai

### 1. Kiểm Tra Cụm Kubernetes

```bash
# Kiểm tra kết nối đến cluster
kubectl cluster-info

# Kiểm tra phiên bản Kubernetes (cần >= 1.19)
kubectl version --short

# Kiểm tra các node
kubectl get nodes
```

### 2. Cài Đặt Helm (nếu chưa có)

```bash
# Trên Windows (PowerShell)
choco install kubernetes-helm

# Hoặc tải từ https://helm.sh/docs/intro/install/
# Sau khi cài, kiểm tra:
helm version
```

### 3. Kiểm Tra Hubble Relay

Hubble Guard cần kết nối đến Hubble Relay. Kiểm tra xem Hubble Relay đã được cài đặt chưa:

```bash
# Kiểm tra namespace hubble
kubectl get namespace hubble

# Kiểm tra service hubble-relay
kubectl get svc -n hubble | grep hubble-relay

# Nếu chưa có, bạn cần cài đặt Cilium và Hubble trước
# Xem: https://docs.cilium.io/en/stable/gettingstarted/hubble/
```

**Lưu ý**: Địa chỉ Hubble Relay mặc định là `hubble-relay.hubble.svc.cluster.local:4245`. Nếu cluster của bạn có cấu hình khác, bạn cần cập nhật trong `values.yaml`.

## 🚀 Các Bước Triển Khai

### Bước 1: Chuẩn Bị File Cấu Hình

Tạo file `my-values.yaml` để override các giá trị mặc định cho cluster của bạn:

```yaml
# my-values.yaml
# Cấu hình ứng dụng
application:
  # Địa chỉ Hubble Relay - THAY ĐỔI theo cluster của bạn
  hubble_server: "hubble-relay.hubble.svc.cluster.local:4245"
  prometheus_export_url: "8080"
  default_namespace: "default"
  auto_start: false

# Cấu hình Anomaly Detector
anomalyDetector:
  image:
    # Thay đổi registry nếu cần
    repository: docker.io/ramseytrinh338/hubble-guard
    tag: "1.0.0"
  
  resources:
    limits:
      cpu: 1000m
      memory: 512Mi
    requests:
      cpu: 100m
      memory: 128Mi

# Cấu hình Prometheus
prometheus:
  persistence:
    enabled: true
    size: 10Gi
    # Thay đổi storageClass nếu cluster của bạn có storage class khác
    storageClass: ""  # Để trống để dùng default storage class
  
  resources:
    limits:
      cpu: 1000m
      memory: 2Gi
    requests:
      cpu: 500m
      memory: 1Gi

# Cấu hình Grafana
grafana:
  adminUser: "admin"
  # THAY ĐỔI mật khẩu mặc định
  adminPassword: "your-secure-password-here"
  
  persistence:
    enabled: false  # Bật lên nếu muốn lưu dashboards
    size: 10Gi
    storageClass: ""

# Cấu hình Alerting (tùy chọn)
alerting:
  enabled: true
  telegram:
    bot_token: ""  # Điền token nếu muốn dùng Telegram
    chat_id: ""     # Điền chat ID nếu muốn dùng Telegram
    enabled: false  # Bật lên sau khi điền token và chat_id

# Namespaces cần monitor
namespaces:
  - "default"
  - "kube-system"
```

### Bước 2: Kiểm Tra Helm Chart

```bash
# Di chuyển vào thư mục project
cd /path/to/final

# Kiểm tra cú pháp Helm chart
helm lint ./helm/hubble-guard

# Xem trước các manifests sẽ được tạo (không thực sự deploy)
helm template hubble-guard ./helm/hubble-guard -f my-values.yaml
```

### Bước 3: Triển Khai với Helm

```bash
# Cách 1: Triển khai trực tiếp từ thư mục chart (Khuyến nghị)
helm install hubble-guard ./helm/hubble-guard \
  -n hubble-guard \
  --create-namespace \
  -f my-values.yaml

# Cách 2: Nếu muốn đặt tên release khác
helm install my-hubble-guard ./helm/hubble-guard \
  -n hubble-guard \
  --create-namespace \
  -f my-values.yaml
```

### Bước 4: Kiểm Tra Triển Khai

```bash
# Kiểm tra status của Helm release
helm status hubble-guard -n hubble-guard

# Kiểm tra các pods
kubectl get pods -n hubble-guard

# Kiểm tra các services
kubectl get svc -n hubble-guard

# Xem logs của Anomaly Detector
kubectl logs -n hubble-guard -l app.kubernetes.io/component=anomaly-detector -f

# Xem logs của Prometheus
kubectl logs -n hubble-guard -l app.kubernetes.io/component=prometheus -f

# Xem logs của Grafana
kubectl logs -n hubble-guard -l app.kubernetes.io/component=grafana -f
```

### Bước 5: Truy Cập Services

#### Cách 1: Port Forwarding (Để test nhanh)

```bash
# Prometheus
kubectl port-forward -n hubble-guard svc/hubble-guard-prometheus 9090:9090
# Truy cập: http://localhost:9090

# Grafana
kubectl port-forward -n hubble-guard svc/hubble-guard-grafana 3000:3000
# Truy cập: http://localhost:3000
# Username: admin
# Password: (mật khẩu bạn đã đặt trong my-values.yaml)

# Anomaly Detector metrics
kubectl port-forward -n hubble-guard svc/hubble-guard-anomaly-detector 8080:8080
# Truy cập: http://localhost:8080/metrics
```

#### Cách 2: Sử dụng Service URLs trong Cluster

- **Anomaly Detector**: `http://hubble-guard-anomaly-detector.hubble-guard.svc.cluster.local:8080`
- **Prometheus**: `http://hubble-guard-prometheus.hubble-guard.svc.cluster.local:9090`
- **Grafana**: `http://hubble-guard-grafana.hubble-guard.svc.cluster.local:3000`

#### Cách 3: Cấu Hình Ingress (Cho Production)

Nếu muốn truy cập từ bên ngoài cluster, bạn cần thêm Ingress. Tạo file `ingress-values.yaml`:

```yaml
# Thêm vào my-values.yaml hoặc tạo file riêng
ingress:
  enabled: true
  className: "nginx"  # hoặc ingress controller của bạn
  annotations:
    cert-manager.io/cluster-issuer: "letsencrypt-prod"
  hosts:
    - host: grafana.yourdomain.com
      paths:
        - path: /
          pathType: Prefix
  tls:
    - secretName: grafana-tls
      hosts:
        - grafana.yourdomain.com
```

## 🔧 Cập Nhật Cấu Hình

### Upgrade Release

```bash
# Sau khi chỉnh sửa my-values.yaml
helm upgrade hubble-guard ./helm/hubble-guard \
  -n hubble-guard \
  -f my-values.yaml

# Hoặc upgrade với nhiều values files
helm upgrade hubble-guard ./helm/hubble-guard \
  -n hubble-guard \
  -f my-values.yaml \
  -f production-overrides.yaml
```

### Xem Lịch Sử và Rollback

```bash
# Xem lịch sử releases
helm history hubble-guard -n hubble-guard

# Rollback về phiên bản trước
helm rollback hubble-guard -n hubble-guard

# Rollback về phiên bản cụ thể (ví dụ: revision 2)
helm rollback hubble-guard 2 -n hubble-guard
```

## 🗑️ Gỡ Cài Đặt

```bash
# Uninstall Helm release
helm uninstall hubble-guard -n hubble-guard

# Xóa namespace (tùy chọn)
kubectl delete namespace hubble-guard
```

## 🔍 Troubleshooting

### 1. Pods Không Khởi Động

```bash
# Kiểm tra events
kubectl describe pod <pod-name> -n hubble-guard

# Kiểm tra logs
kubectl logs <pod-name> -n hubble-guard

# Kiểm tra ConfigMap
kubectl get configmap -n hubble-guard -o yaml
```

### 2. Anomaly Detector Không Kết Nối Được Hubble

```bash
# Kiểm tra địa chỉ Hubble Relay
kubectl get svc -n hubble | grep hubble-relay

# Test kết nối từ pod
kubectl exec -n hubble-guard <anomaly-detector-pod> -- \
  nc -zv hubble-relay.hubble.svc.cluster.local 4245

# Kiểm tra network policies
kubectl get networkpolicies -n hubble-guard
kubectl get networkpolicies -n hubble
```

### 3. Prometheus Không Scrape Được Metrics

```bash
# Kiểm tra Prometheus config
kubectl get configmap -n hubble-guard hubble-guard-prometheus -o yaml

# Xem targets trong Prometheus UI
# Truy cập: http://localhost:9090/targets (sau khi port-forward)

# Kiểm tra service selector
kubectl get svc -n hubble-guard hubble-guard-anomaly-detector -o yaml
```

### 4. Grafana Không Hiển Thị Dashboard

```bash
# Kiểm tra datasource provisioning
kubectl get configmap -n hubble-guard | grep grafana

# Xem logs Grafana
kubectl logs -n hubble-guard -l app.kubernetes.io/component=grafana

# Kiểm tra dashboard ConfigMap
kubectl get configmap hubble-guard-grafana-dashboard -n hubble-guard -o yaml
```

### 5. Lỗi Image Pull

```bash
# Kiểm tra image pull secrets
kubectl get secrets -n hubble-guard

# Nếu cần, thêm image pull secret vào values.yaml:
anomalyDetector:
  imagePullSecrets:
    - name: my-registry-secret
```

## 📝 Cấu Hình Cho Các Môi Trường Khác Nhau

### Development

```yaml
# dev-values.yaml
anomalyDetector:
  resources:
    limits:
      cpu: 500m
      memory: 256Mi

prometheus:
  persistence:
    enabled: false  # Không cần persistence cho dev

grafana:
  adminPassword: "dev-password"
```

### Production

```yaml
# prod-values.yaml
anomalyDetector:
  replicaCount: 2  # High availability
  resources:
    limits:
      cpu: 2000m
      memory: 1Gi

prometheus:
  persistence:
    enabled: true
    size: 50Gi
    storageClass: "fast-ssd"
  retention: "30d"

grafana:
  adminPassword: "secure-production-password"
  persistence:
    enabled: true
    size: 20Gi

alerting:
  telegram:
    enabled: true
    bot_token: "PROD_BOT_TOKEN"
    chat_id: "PROD_CHAT_ID"
```

## 🔐 Bảo Mật

### 1. Sử dụng Secrets cho Sensitive Data

```bash
# Tạo secret cho Grafana password
kubectl create secret generic grafana-admin \
  --from-literal=admin-password='your-secure-password' \
  -n hubble-guard

# Tạo secret cho Telegram bot token
kubectl create secret generic telegram-secret \
  --from-literal=bot-token='YOUR_BOT_TOKEN' \
  --from-literal=chat-id='YOUR_CHAT_ID' \
  -n hubble-guard
```

Sau đó cập nhật values.yaml để sử dụng secrets (cần chỉnh sửa Helm templates).

### 2. RBAC

Chart đã tạo ServiceAccount. Nếu cần thêm quyền, tạo Role và RoleBinding:

```yaml
# rbac.yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: hubble-guard-role
  namespace: hubble-guard
rules:
  - apiGroups: [""]
    resources: ["pods", "services"]
    verbs: ["get", "list", "watch"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: hubble-guard-rolebinding
  namespace: hubble-guard
subjects:
  - kind: ServiceAccount
    name: hubble-guard-anomaly-detector
    namespace: hubble-guard
roleRef:
  kind: Role
  name: hubble-guard-role
  apiGroup: rbac.authorization.k8s.io
```

## 📚 Tài Liệu Tham Khảo

- [Helm Documentation](https://helm.sh/docs/)
- [Kubernetes Documentation](https://kubernetes.io/docs/)
- [Cilium Hubble Documentation](https://docs.cilium.io/en/stable/gettingstarted/hubble/)
- [Prometheus Documentation](https://prometheus.io/docs/)
- [Grafana Documentation](https://grafana.com/docs/)

## ✅ Checklist Triển Khai

- [ ] Kubernetes cluster đã sẵn sàng (>= 1.19)
- [ ] Helm 3.0+ đã cài đặt
- [ ] kubectl đã cấu hình và kết nối được cluster
- [ ] Hubble Relay đã được cài đặt và chạy
- [ ] Đã tạo file `my-values.yaml` với cấu hình phù hợp
- [ ] Đã kiểm tra Helm chart (`helm lint`)
- [ ] Đã triển khai thành công (`helm install`)
- [ ] Tất cả pods đang chạy (`kubectl get pods`)
- [ ] Có thể truy cập Prometheus (port-forward hoặc service)
- [ ] Có thể truy cập Grafana và đăng nhập được
- [ ] Anomaly Detector đang kết nối được với Hubble Relay
- [ ] Prometheus đang scrape được metrics từ Anomaly Detector
- [ ] Grafana dashboard hiển thị dữ liệu

---

**Lưu ý**: Nếu gặp vấn đề, hãy kiểm tra logs và events của các pods để tìm nguyên nhân. Phần Troubleshooting ở trên sẽ giúp bạn giải quyết các vấn đề phổ biến.

