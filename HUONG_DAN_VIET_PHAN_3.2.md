# HƯỚNG DẪN VIẾT PHẦN 3.2: TRIỂN KHAI CỤ THỂ

## Mục đích của phần này

Phần 3.2 "Triển khai cụ thể" là phần quan trọng trong đồ án tốt nghiệp, thể hiện khả năng triển khai thực tế của sinh viên. Phần này cần trình bày:

1. **Kiến trúc tổng quan** của hệ thống đã xây dựng
2. **Các thành phần chính** trong source code
3. **Cấu hình và triển khai** các rules phát hiện bất thường
4. **Luồng xử lý dữ liệu** từ thu thập đến cảnh báo
5. **Các điểm kỹ thuật quan trọng** trong quá trình triển khai

---

## 3.2.1. Kiến trúc tổng quan hệ thống

### 3.2.1.1. Mô hình kiến trúc

Hệ thống phát hiện bất thường mạng được xây dựng theo mô hình **kiến trúc phân tầng** với các thành phần chính:

```
┌─────────────────────────────────────────────────────────────┐
│                    Hubble Anomaly Detector                   │
├─────────────────────────────────────────────────────────────┤
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐      │
│  │ Data Layer   │  │ Process Layer │  │ Alert Layer  │      │
│  │              │  │               │  │              │      │
│  │ - Hubble     │  │ - Rule Engine │  │ - Telegram   │      │
│  │   Client     │  │ - Metrics     │  │ - Log        │      │
│  │ - Prometheus │  │   Collector   │  │ - Webhook    │      │
│  │   Client     │  │               │  │              │      │
│  └──────────────┘  └──────────────┘  └──────────────┘      │
└─────────────────────────────────────────────────────────────┘
         │                    │                    │
         ▼                    ▼                    ▼
┌──────────────┐    ┌──────────────┐    ┌──────────────┐
│   Hubble     │    │  Prometheus  │    │  Alerting    │
│   Relay      │    │   Server     │    │  Channels    │
└──────────────┘    └──────────────┘    └──────────────┘
```

### 3.2.1.2. Cấu trúc thư mục dự án

Dự án được tổ chức theo cấu trúc modular, tuân thủ best practices của Go:

```
final/
├── cmd/
│   └── hubble-detector/
│       └── main.go              # Entry point của ứng dụng
├── internal/
│   ├── client/                  # Client layer
│   │   ├── hubble_client.go     # Hubble gRPC client
│   │   ├── prometheus_client.go # Prometheus query client
│   │   └── metrics_collector.go # Metrics collection
│   ├── rules/                   # Rule engine
│   │   ├── engine.go            # Rule engine core
│   │   └── builtin/             # Built-in rules
│   │       ├── traffic_spike_rule.go
│   │       ├── port_scan_rule.go
│   │       ├── block_connection_rule.go
│   │       ├── namespace_access_rule.go
│   │       └── ...
│   ├── alert/                   # Alerting system
│   │   ├── telegram.go
│   │   ├── log_alert.go
│   │   └── prometheus.go
│   ├── model/                  # Data models
│   │   ├── flow.go
│   │   └── rule.go
│   └── utils/                  # Utilities
│       ├── config_loader.go
│       └── logger.go
├── configs/
│   └── anomaly_detection.yaml  # Cấu hình chính
└── helm/                       # Kubernetes deployment
    └── hubble-guard/
```

**Giải thích cấu trúc:**
- `cmd/`: Chứa các entry point của ứng dụng, tuân thủ Go project layout
- `internal/`: Chứa code nội bộ, không được import bởi các package khác
- `configs/`: Chứa các file cấu hình YAML
- `helm/`: Chứa Helm charts cho việc triển khai trên Kubernetes

---

## 3.2.2. Các thành phần chính của source code

### 3.2.2.1. Entry Point (main.go)

**Vị trí:** `cmd/hubble-detector/main.go`

**Chức năng:**
- Khởi tạo và cấu hình hệ thống
- Load cấu hình từ file YAML
- Khởi tạo các client (Hubble, Prometheus)
- Đăng ký các rules và notifiers
- Bắt đầu luồng thu thập và xử lý dữ liệu

**Điểm quan trọng cần trình bày:**

```go
// 1. Load cấu hình từ YAML
yamlConfig, err := utils.LoadAnomalyDetectionConfig(*configFile)
if err != nil {
    // Fallback to default config
    config = utils.GetDefaultConfig()
} else {
    config = yamlConfig.ToDefaultConfig()
}

// 2. Khởi tạo Prometheus exporter
exporter, err := alert.StartPrometheusExporterWithCustomRegistry(
    prometheusPort, logger)

// 3. Khởi tạo Hubble client với metrics
hubbleClient, err := client.NewHubbleGRPCClientWithMetrics(
    hubbleServer, exporter.GetMetrics())

// 4. Khởi tạo Prometheus client
promClient, err := client.NewPrometheusClient(config.Prometheus.URL)

// 5. Khởi tạo Rule Engine
engine := rules.NewEngine(logger)

// 6. Đăng ký các built-in rules từ YAML
utils.RegisterBuiltinRulesFromYAML(engine, yamlConfig, logger, promClient)

// 7. Đăng ký các alert notifiers
registerAlertNotifiers(engine, config, yamlConfig, logger)

// 8. Bắt đầu stream flows từ Hubble
streamFlowsToPrometheus(hubbleClient, namespaces, engine, logger, config)
```

**Giải thích:**
- Hệ thống hỗ trợ cấu hình linh hoạt qua YAML, có fallback về default config
- Tách biệt rõ ràng giữa data collection, processing và alerting
- Sử dụng dependency injection để dễ dàng test và mở rộng

### 3.2.2.2. Hubble Client (hubble_client.go)

**Vị trí:** `internal/client/hubble_client.go`

**Chức năng:**
- Kết nối đến Hubble Relay qua gRPC
- Stream flows real-time từ Hubble
- Chuyển đổi Hubble Flow format sang internal Flow model
- Ghi lại metrics cho Prometheus

**Điểm kỹ thuật quan trọng:**

```go
// Kết nối gRPC đến Hubble Relay
conn, err := grpc.Dial(server, 
    grpc.WithTransportCredentials(insecure.NewCredentials()))

req := &observer.GetFlowsRequest{
    Follow: true,
    Whitelist: []*observer.FlowFilter{
        {
            SourceLabel: []string{
                "k8s:io.kubernetes.pod.namespace=" + namespace
            },
        },
    },
}

flow := c.convertHubbleFlow(response.GetFlow())
if flow != nil {
    c.metrics.RecordFlow(flow)
    c.recordAnomalyDetectionMetrics(flow)
}
```

**Giải thích:**
- Sử dụng gRPC streaming để nhận flows real-time, không cần polling
- Filter flows theo namespace để giảm tải xử lý
- Tách biệt conversion logic để dễ maintain và test

### 3.2.2.3. Metrics Collector (metrics_collector.go)

**Vị trí:** `internal/client/metrics_collector.go`

**Chức năng:**
- Định nghĩa và quản lý các Prometheus metrics
- Ghi lại metrics từ flows
- Hỗ trợ các metrics cho anomaly detection

**Các metrics quan trọng:**

```go
// Flow metrics
hubble_flows_total{namespace}
hubble_flows_by_verdict_total{verdict, namespace}

// Anomaly detection metrics
portscan_distinct_ports_10s{source_ip, dest_ip}
hubble_namespace_access_total{source_namespace, dest_namespace, ...}
hubble_suspicious_outbound_total{namespace, destination_port}
hubble_tcp_drops_total{namespace, source_ip, destination_ip}
```

**Giải thích:**
- Metrics được tổ chức theo namespace để dễ query và filter
- Sử dụng Counter, Gauge, Histogram phù hợp với từng loại metric
- Port scan tracking sử dụng time-windowed tracking (10 giây)

### 3.2.2.4. Rule Engine (engine.go)

**Vị trí:** `internal/rules/engine.go`

**Chức năng:**
- Quản lý và đánh giá các rules
- Phát sinh alerts khi phát hiện bất thường
- Gửi alerts đến các notifiers

**Kiến trúc Rule Engine:**

```go
type Engine struct {
    rules          []RuleInterface
    alertNotifiers []NotifierInterface
    logger         *logrus.Logger
    mu             sync.RWMutex
    alertChannel   chan model.Alert
}

type RuleInterface interface {
    Name() string
    IsEnabled() bool
    Evaluate(ctx context.Context, flow *model.Flow) *model.Alert
    Start(ctx context.Context)  // Cho Prometheus-based rules
}
```

**Giải thích:**
- Sử dụng interface để dễ dàng thêm rules mới
- Thread-safe với mutex để bảo vệ shared state
- Alert channel để decouple rule evaluation và alert delivery

---

## 3.2.3. Cấu hình các Rules phát hiện bất thường

### 3.2.3.1. Cấu trúc file cấu hình

**File:** `configs/anomaly_detection.yaml`

File cấu hình sử dụng định dạng YAML, cho phép cấu hình linh hoạt các rules:

```yaml
application:
  hubble_server: "localhost:4245"
  prometheus_export_url: "8080"
  default_namespace: "default"

prometheus:
  url: "http://localhost:9090"
  timeout_seconds: 10

namespaces:
  - "default"
  - "test"

rules:
  - name: "traffic_spike"
    enabled: true
    severity: "CRITICAL"
    description: "Phát hiện traffic spike có thể là DDoS"
    thresholds:
      multiplier: 3.0
  
  - name: "port_scan"
    enabled: true
    severity: "HIGH"
    description: "Phát hiện port scanning attacks"
    thresholds:
      distinct_ports: 10
```

### 3.2.3.2. Rule 1: Traffic Spike Detection (DDoS)

**File implementation:** `internal/rules/builtin/traffic_spike_rule.go`

**Mục đích:** Phát hiện tấn công DDoS dựa trên sự tăng đột biến traffic

**Cơ chế hoạt động:**

1. **Baseline Collection (1 phút đầu):**
   - Thu thập traffic rate trong 1 phút để tính baseline
   - Tính trung bình các samples trong window

2. **Anomaly Detection:**
   - Query Prometheus: `rate(hubble_flows_total{namespace="X"}[1m])`
   - So sánh current rate với baseline
   - Alert nếu: `current_rate > baseline * threshold`

**Cấu hình:**

```yaml
rules:
  - name: "traffic_spike"
    enabled: true
    severity: "CRITICAL"
    thresholds:
      multiplier: 3.0  # Alert nếu traffic > 3x baseline
```

**Code quan trọng:**

```go
// Tính baseline
baseline := avg(baselineRates)  // Trung bình trong 1 phút

// So sánh
multiplier := currentRate / baseline
if multiplier > r.threshold {
    // Phát sinh alert
    alert := &model.Alert{
        Type:      "ddos_traffic_spike",
        Severity:  "CRITICAL",
        Message:   fmt.Sprintf("Traffic spike: %.2fx baseline", multiplier),
    }
}
```

**Ưu điểm:**
- Tự động học baseline, không cần cấu hình thủ công
- Phát hiện được cả DDoS từ nhiều nguồn
- Có thể điều chỉnh sensitivity qua threshold

### 3.2.3.3. Rule 2: Port Scan Detection

**File implementation:** `internal/rules/builtin/port_scan_rule.go`

**Mục đích:** Phát hiện port scanning attacks

**Cơ chế hoạt động:**

1. **Port Tracking:**
   - Track distinct destination ports cho mỗi source-dest pair
   - Time window: 10 giây
   - Metric: `portscan_distinct_ports_10s{source_ip, dest_ip}`

2. **Detection:**
   - Query Prometheus: `portscan_distinct_ports_10s > threshold`
   - Alert nếu số lượng distinct ports vượt ngưỡng

**Cấu hình:**

```yaml
rules:
  - name: "port_scan"
    enabled: true
    severity: "HIGH"
    thresholds:
      distinct_ports: 10  # Alert nếu > 10 ports trong 10s
```

**Code quan trọng:**

```go
// Track ports trong time window
func (pst *portScanTracker) addPort(sourceIP, destIP string, port uint16) {
    key := fmt.Sprintf("%s:%s", sourceIP, destIP)
    entry.ports[port] = time.Now()
    // Cleanup ports cũ hơn 10s
    cleanupOldPorts(key, entry, now)
}

// Query và check
query := `portscan_distinct_ports_10s > 0`
if distinctPorts > r.threshold {
    // Alert
}
```

**Ưu điểm:**
- Phát hiện nhanh (10 giây window)
- Chính xác (track theo source-dest pair)
- Giảm false positive nhờ time window

### 3.2.3.4. Rule 3: Block Connection Detection

**File implementation:** `internal/rules/builtin/block_connection_rule.go`

**Mục đích:** Phát hiện các kết nối bị chặn (DROP flows)

**Cơ chế hoạt động:**

1. **Metrics Collection:**
   - Track DROP flows: `hubble_flows_by_verdict_total{verdict="DROP", namespace}`

2. **Detection:**
   - Query: `sum(increase(hubble_flows_by_verdict_total{verdict="DROP"}[1m]))`
   - Alert nếu số lượng DROP flows vượt ngưỡng trong 1 phút

**Cấu hình:**

```yaml
rules:
  - name: "block_connection"
    enabled: true
    severity: "HIGH"
    thresholds:
      per_minute: 10  # Alert nếu > 10 DROP flows/phút
```

**Giải thích:**
- DROP flows có thể do network policy, firewall rules
- Số lượng lớn DROP flows có thể chỉ ra:
  - Tấn công bị chặn
  - Cấu hình sai network policy
  - Service không thể truy cập

### 3.2.3.5. Rule 4: Namespace Access Detection

**File implementation:** `internal/rules/builtin/namespace_access_rule.go`

**Mục đích:** Phát hiện truy cập trái phép đến namespace nhạy cảm

**Cơ chế hoạt động:**

1. **Cross-namespace Tracking:**
   - Track: `hubble_namespace_access_total{source_namespace, dest_namespace, ...}`

2. **Detection:**
   - Query các forbidden namespaces (kube-system, monitoring, security)
   - Alert nếu có traffic từ namespace khác đến forbidden namespace

**Cấu hình:**

```yaml
rules:
  - name: "namespace_access"
    enabled: true
    severity: "HIGH"
    thresholds:
      forbidden_namespaces:
        - "kube-system"
        - "monitoring"
        - "security"
```

**Code quan trọng:**

```go
// Query cho forbidden namespace
query := fmt.Sprintf(
    `sum(increase(hubble_namespace_access_total{dest_namespace="%s"}[1m])) 
     by (source_namespace, dest_namespace)`,
    forbiddenNSName)

// Check và alert
if forbiddenNS[destNS] && sourceNS != destNS {
    // Phát hiện truy cập trái phép
    alert := &model.Alert{
        Message: fmt.Sprintf(
            "Unauthorized access: %s -> %s",
            sourceNS, destNS),
    }
}
```

**Giải thích:**
- Bảo vệ các namespace nhạy cảm khỏi truy cập trái phép
- Có thể phát hiện lateral movement trong cluster
- Đặc biệt chú ý đến kube-dns access

### 3.2.3.6. Rule 5: Suspicious Outbound Detection

**File implementation:** `internal/rules/builtin/outbound_rule.go`

**Mục đích:** Phát hiện kết nối outbound đáng ngờ đến các port nguy hiểm

**Các port được theo dõi:**
- Port 23 (Telnet)
- Port 135 (RPC)
- Port 445 (SMB)
- Port 1433 (SQL Server)
- Port 3306 (MySQL)
- Port 5432 (PostgreSQL)

**Cấu hình:**

```yaml
rules:
  - name: "suspicious_outbound"
    enabled: true
    severity: "HIGH"
    thresholds:
      per_minute: 10
```

**Giải thích:**
- Các port này thường được sử dụng trong attacks
- Phát hiện data exfiltration hoặc lateral movement
- Có thể chỉ ra compromised pod

---

## 3.2.4. Luồng xử lý dữ liệu

### 3.2.4.1. Data Flow Diagram

```
Hubble Relay
    │
    │ (gRPC Stream)
    ▼
Hubble Client
    │
    │ (Convert Flow)
    ▼
Metrics Collector
    │
    │ (Record Metrics)
    ▼
Prometheus Server
    │
    │ (Query Metrics)
    ▼
Rule Engine
    │
    │ (Evaluate Rules)
    ▼
Alert Generation
    │
    ├──► Telegram Notifier
    ├──► Log Notifier
    └──► Webhook Notifier
```

### 3.2.2.2. Chi tiết các bước xử lý

**Bước 1: Thu thập dữ liệu từ Hubble**

```go
// Stream flows từ Hubble Relay
stream, err := client.GetFlows(ctx, req)
for {
    response, err := stream.Recv()
    flow := convertHubbleFlow(response.GetFlow())
    // Xử lý flow
}
```

**Bước 2: Ghi metrics vào Prometheus**

```go
// Ghi các metrics
metrics.RecordFlow(flow)
metrics.RecordTCPReset(namespace, sourceIP, destIP)
metrics.UpdatePortScanDistinctPorts(sourceIP, destIP, port)
```

**Bước 3: Rule Engine query Prometheus**

```go
// Mỗi rule chạy trong goroutine riêng
go rule.Start(ctx)

// Rule query Prometheus định kỳ (mỗi 10 giây)
ticker := time.NewTicker(10 * time.Second)
for {
    <-ticker.C
    result := prometheusAPI.Query(ctx, query)
    // Đánh giá và phát sinh alert
}
```

**Bước 4: Gửi alerts**

```go
// Engine emit alert
engine.EmitAlert(alert)

// Gửi đến các notifiers
for _, notifier := range notifiers {
    notifier.SendAlert(alert)
}
```

---

## 3.2.5. Các điểm kỹ thuật quan trọng

### 3.2.5.1. Thread Safety

- Sử dụng `sync.RWMutex` để bảo vệ shared state
- Rule Engine sử dụng mutex khi đăng ký rules
- Metrics collector thread-safe nhờ Prometheus client

### 3.2.5.2. Error Handling

- Graceful degradation: Nếu Prometheus không available, rules vẫn chạy nhưng không query
- Retry mechanism cho Prometheus queries
- Logging chi tiết để debug

### 3.2.5.3. Performance Optimization

- gRPC streaming thay vì polling
- Time-windowed tracking cho port scan (tránh memory leak)
- Batch processing cho metrics

### 3.2.5.4. Configuration Management

- YAML-based configuration
- Environment variable support
- Default values cho tất cả settings
- Validation khi load config

---

## 3.2.6. Triển khai trên Kubernetes

### 3.2.6.1. Helm Chart Structure

```
helm/hubble-guard/
├── Chart.yaml
├── values.yaml
└── templates/
    ├── anomaly-detector-deploy.yaml
    ├── anomaly-detector-svc.yaml
    ├── anomaly-detector-configmap.yaml
    └── ...
```

### 3.2.6.2. Deployment Configuration

**ConfigMap cho cấu hình:**

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: anomaly-detector-config
data:
  anomaly_detection.yaml: |
    application:
      hubble_server: "hubble-relay.hubble.svc.cluster.local:4245"
    prometheus:
      url: "http://prometheus-server.monitoring.svc.cluster.local:9090"
    rules:
      - name: "traffic_spike"
        enabled: true
        ...
```

**Deployment:**

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: hubble-anomaly-detector
spec:
  replicas: 1
  template:
    spec:
      containers:
      - name: detector
        image: hubble-anomaly-detector:latest
        args:
        - --config=/etc/config/anomaly_detection.yaml
        volumeMounts:
        - name: config
          mountPath: /etc/config
```

---

## 3.2.7. Kết luận phần triển khai

Trong phần này, sinh viên cần:

1. **Trình bày rõ ràng kiến trúc** và lý do lựa chọn
2. **Giải thích từng thành phần** và vai trò của nó
3. **Chi tiết về cấu hình rules** và cách chúng hoạt động
4. **Minh họa bằng code snippets** quan trọng
5. **Giải thích các quyết định kỹ thuật** và trade-offs

**Lưu ý:**
- Không chỉ liệt kê code, mà phải giải thích **tại sao** làm như vậy
- Sử dụng diagrams để minh họa luồng xử lý
- So sánh với các approach khác nếu có
- Nêu rõ các thách thức gặp phải và cách giải quyết

---

## Phụ lục: Template cho phần 3.2

### 3.2.1. Kiến trúc tổng quan hệ thống
- [ ] Mô hình kiến trúc (diagram)
- [ ] Cấu trúc thư mục dự án
- [ ] Giải thích lựa chọn kiến trúc

### 3.2.2. Các thành phần chính
- [ ] Entry Point (main.go)
- [ ] Hubble Client
- [ ] Metrics Collector
- [ ] Rule Engine
- [ ] Alert System

### 3.2.3. Cấu hình Rules
- [ ] Cấu trúc file cấu hình
- [ ] Rule 1: Traffic Spike
- [ ] Rule 2: Port Scan
- [ ] Rule 3: Block Connection
- [ ] Rule 4: Namespace Access
- [ ] Rule 5: Suspicious Outbound

### 3.2.4. Luồng xử lý dữ liệu
- [ ] Data Flow Diagram
- [ ] Chi tiết các bước xử lý
- [ ] Timing và synchronization

### 3.2.5. Điểm kỹ thuật quan trọng
- [ ] Thread Safety
- [ ] Error Handling
- [ ] Performance Optimization
- [ ] Configuration Management

### 3.2.6. Triển khai
- [ ] Kubernetes Deployment
- [ ] Helm Charts
- [ ] Monitoring và Logging

