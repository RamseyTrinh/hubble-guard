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
┌─────────────────────────────────────────────────────────────────────────────┐
│                              Hubble Guard                                    │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  ┌──────────────────┐  ┌──────────────────┐  ┌──────────────────┐  ┌──────┐│
│  │  Data Layer      │  │ Process Layer    │  │  Alert Layer     │  │API   ││
│  │  (Green)         │  │ (Red)            │  │ (Blue)           │  │Server││
│  │                  │  │                  │  │                  │  │(Gray)││
│  │ - Hubble Client  │  │ - Rule Engine    │  │ - Telegram       │  │      ││
│  │ - Prometheus     │  │ - Metrics        │  │ - Log            │  │-REST ││
│  │   Client         │  │   Collector      │  │ - Webhook        │  │-WS   ││
│  │                  │  │                  │  │                  │  │      ││
│  └────────┬─────────┘  └────────┬─────────┘  └────────┬─────────┘  └──┬───┘│
│           │                     │                      │              │     │
│           │                     │                      │              │     │
│  ┌────────┴───────────────────────────────────────────┴──────────────┴───┐ │
│  │                        UI Layer (Yellow)                              │ │
│  │  - Dashboard  - FlowViewer                                             │ │
│  └───────────────────────────────────────────────────────────────────────┘ │
└───────────┼─────────────────────┼──────────────────────┼──────────────┼─────┘
            │                     │                      │              │
            ▼                     ▼                      ▼              ▼
    ┌──────────────┐    ┌──────────────┐    ┌──────────────┐  ┌──────────┐
    │  Hubble      │    │  Prometheus  │    │  Alerting    │  │   Web    │
    │  Relay       │    │  Server      │    │  Channels    │  │ Browser  │
    └──────────────┘    └──────────────┘    └──────────────┘  └──────────┘
```

**Giải thích kiến trúc:**

- **Data Layer (Green)**: Thu thập dữ liệu từ Hubble Relay và Prometheus Server
- **Process Layer (Red)**: Xử lý dữ liệu, đánh giá rules, thu thập metrics
- **Alert Layer (Blue)**: Gửi cảnh báo qua các kênh (Telegram, Log, Webhook)
- **API Server (Gray)**: Cung cấp REST API và WebSocket để UI truy cập dữ liệu
- **UI Layer (Yellow)**: Giao diện web hiển thị dashboard và flow viewer, giao tiếp với API Server

### 3.2.1.2. Cấu trúc thư mục dự án

Dự án được tổ chức theo cấu trúc modular, tuân thủ best practices của Go và frontend development:

```
hubble-guard/
├── cmd/
│   └── hubble-detector/
│       └── main.go              # Entry point của detector
├── api/                         # API Server
├── ui/                          # Web UI (React + Vite)
├── internal/
│   ├── client/                  # All Client layer
│   ├── rules/                   # Rule engine
│   │   ├── engine.go            # Rule engine core
│   │   └── builtin/             # Built-in rules
│   │       ├── traffic_spike_rule.go
│   │       ├── port_scan_rule.go
│   │       ├── block_connection_rule.go
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
│   └── anomaly_detection.yaml
└── helm/                       # Kubernetes deployment
    └── hubble-guard/
        ├── templates/
        │   ├── ui-deploy.yaml   # UI deployment
        │   ├── ui-svc.yaml      # UI service
        │   ├── ui-ingress.yaml  # UI ingress
        │   └── ...
        └── values.yaml
```

**Giải thích cấu trúc:**
- `cmd/`: Chứa các entry point của ứng dụng, tuân thủ Go project layout
- `api/`: API server cung cấp REST API và WebSocket cho UI
- `ui/`: Frontend React application với Vite, deploy riêng biệt
- `internal/`: Chứa code nội bộ, không được import bởi các package khác
- `configs/`: Chứa các file cấu hình YAML
- `helm/`: Chứa Helm charts cho việc triển khai trên Kubernetes, bao gồm cả UI và API

---

## 3.2.2. Các thành phần chính của source code

### 3.2.2.1. Entry Point (main.go)

**Vị trí:** `cmd/hubble-guard/main.go`

**Chức năng:**
- Khởi tạo và cấu hình hệ thống
- Load cấu hình từ file YAML
- Khởi tạo các client (Hubble, Prometheus)
- Đăng ký các rules và notifiers
- Bắt đầu luồng thu thập và xử lý dữ liệu

**Điểm quan trọng:**
- Load cấu hình từ YAML với validation và fallback về default config
- Khởi tạo Prometheus exporter để expose metrics
- Khởi tạo Hubble client với metrics integration
- Đăng ký rules từ YAML config, mỗi rule có thể chạy real-time hoặc query Prometheus định kỳ

### 3.2.2.2. API Server (api/main.go)

**Vị trí:** `api/main.go`

**Chức năng:**
- Cung cấp REST API endpoints cho UI
- WebSocket streaming cho real-time updates
- In-memory storage cho flows, alerts, rules
- Kết nối với Hubble để stream flows cho UI

**Kiến trúc API Server:**

```go
// Khởi tạo in-memory storage
store := storage.NewStorage(logger)

// Khởi tạo Flow Broadcaster (stream flows qua WebSocket)
handlers.InitFlowBroadcaster(hubbleClient, store, config, logger, sharedMetrics)

// Tạo HTTP handlers
h := handlers.NewHandlers(store, config, logger, promClient)

// Setup router với REST API endpoints
api := router.PathPrefix("/api/v1").Subrouter()
api.HandleFunc("/flows", h.GetFlows).Methods("GET")
api.HandleFunc("/alerts", h.GetAlerts).Methods("GET")
api.HandleFunc("/rules", h.GetRules).Methods("GET")
api.HandleFunc("/stream/flows", h.StreamFlows).Methods("GET")  // WebSocket
api.HandleFunc("/stream/alerts", h.StreamAlerts).Methods("GET") // WebSocket
```

**Các API Endpoints chính:**

1. **Flows API:**
   - `GET /api/v1/flows` - Lấy danh sách flows với pagination và filter
   - `GET /api/v1/flows/{id}` - Chi tiết flow
   - `GET /api/v1/flows/stats` - Thống kê flows
   - `WS /api/v1/stream/flows` - WebSocket stream flows real-time

2. **Alerts API:**
   - `GET /api/v1/alerts` - Lấy danh sách alerts với filter
   - `GET /api/v1/alerts/{id}` - Chi tiết alert
   - `GET /api/v1/alerts/timeline` - Timeline alerts
   - `WS /api/v1/stream/alerts` - WebSocket stream alerts real-time

3. **Rules API:**
   - `GET /api/v1/rules` - Lấy danh sách rules
   - `GET /api/v1/rules/{id}` - Chi tiết rule
   - `PUT /api/v1/rules/{id}` - Cập nhật rule (enable/disable, severity)
   - `GET /api/v1/rules/stats` - Thống kê rules

4. **Metrics API:**
   - `GET /api/v1/metrics/prometheus/stats` - Metrics tổng quan từ Prometheus
   - `GET /api/v1/metrics/prometheus/dropped-flows/timeseries` - Time series dropped flows
   - `GET /api/v1/metrics/prometheus/alert-types` - Thống kê alert types

**Flow Broadcaster (WebSocket):**

```go
// FlowBroadcaster stream flows từ Hubble đến tất cả WebSocket clients
type FlowBroadcaster struct {
    clients      map[*websocket.Conn]bool
    hubbleClient *client.HubbleGRPCClient
    store        *storage.Storage
}

// Một gRPC stream duy nhất cho tất cả WebSocket clients
func (fb *FlowBroadcaster) run() {
    stream := fb.hubbleClient.GetFlows(ctx, req)
    for {
        flow := stream.Recv()
        fb.store.AddFlow(flow)  // Lưu vào storage
        fb.broadcastToClients(flow)  // Broadcast đến WebSocket clients
    }
}
```

**Giải thích:**
- API Server chạy độc lập với detector, có Hubble Client riêng để stream flows
- Sử dụng in-memory storage để lưu flows, alerts, rules (tối đa 50k flows, 10k alerts)
- FlowBroadcaster quản lý một gRPC stream duy nhất và broadcast đến nhiều WebSocket clients
- REST API cung cấp CRUD operations, WebSocket cung cấp real-time updates

### 3.2.2.3. UI (Web Interface)

**Vị trí:** `ui/src/`

**Công nghệ:**
- **Framework**: React 18+ với JavaScript
- **Build tool**: Vite
- **UI Library**: Material-UI (MUI)
- **State Management**: Zustand
- **API Client**: Axios
- **Charts**: Recharts
- **Routing**: React Router

**Cấu trúc UI:**

```
ui/src/
├── App.jsx              # Main React component với routing
├── main.jsx             # Entry point
├── components/          # React components
│   ├── Layout.jsx       # Layout với navigation sidebar
│   └── GrafanaEmbed.jsx # Embed Grafana dashboard
├── pages/               # Page components
│   ├── Dashboard.jsx    # Dashboard page - hiển thị metrics, alerts
│   └── FlowViewer.jsx   # Flow viewer page - hiển thị flows
├── services/            # API services
│   └── api.js           # API client (REST + WebSocket)
└── store/               # State management
    └── useStore.js      # Zustand store
```

**Các tính năng chính:**

1. **Dashboard Page (`Dashboard.jsx`):**
   - Hiển thị metrics tổng quan: total flows, alerts, rules
   - Biểu đồ traffic theo thời gian (LineChart)
   - Phân bổ flows theo namespace, verdict (PieChart)
   - Embed Grafana dashboard
   - Real-time updates qua WebSocket

2. **Flow Viewer Page (`FlowViewer.jsx`):**
   - Bảng flows với pagination
   - Filter flows theo namespace, verdict, search
   - Chi tiết từng flow
   - Export flows

3. **API Client (`services/api.js`):**
   - Axios instance với base URL từ environment variable
   - Các API methods: `flowsAPI`, `alertsAPI`, `rulesAPI`, `metricsAPI`
   - WebSocket helper: `createWebSocket()`

**Code quan trọng:**

```javascript
// API Client
const API_BASE_URL = import.meta.env.VITE_API_URL || 'http://localhost:5001/api/v1'
const WS_BASE_URL = import.meta.env.VITE_WS_URL || 'ws://localhost:5001/api/v1'

export const flowsAPI = {
  getAll: (params = {}) => api.get('/flows', { params }),
  getStats: () => api.get('/flows/stats'),
}

// WebSocket connection
export const createWebSocket = (endpoint) => {
  const wsUrl = `${WS_BASE_URL}${endpoint}`
  return new WebSocket(wsUrl)
}
```

**Real-time Updates:**

```javascript
// Trong Dashboard component
useEffect(() => {
  const ws = createWebSocket('/stream/alerts')
  ws.onmessage = (event) => {
    const alert = JSON.parse(event.data)
    // Update UI với alert mới
    updateAlerts(alert)
  }
  return () => ws.close()
}, [])
```

**Tương tác với API Server:**

```javascript
// 1. REST API calls
const fetchFlows = async () => {
  const response = await flowsAPI.getAll({ 
    page: 1, 
    limit: 25, 
    namespace: 'default' 
  })
  setFlows(response.data)
}

// 2. WebSocket cho real-time updates
useEffect(() => {
  const ws = createWebSocket('/stream/alerts')
  
  ws.onopen = () => {
    console.log('WebSocket connected')
  }
  
  ws.onmessage = (event) => {
    const alert = JSON.parse(event.data)
    // Update state với alert mới
    addAlert(alert)
  }
  
  ws.onerror = (error) => {
    console.error('WebSocket error:', error)
  }
  
  ws.onclose = () => {
    console.log('WebSocket disconnected')
    // Reconnect logic có thể thêm ở đây
  }
  
  return () => {
    ws.close()
  }
}, [])
```

**Giải thích:**
- UI là Single Page Application (SPA) với React Router
- Giao tiếp với API Server qua REST API và WebSocket
- Environment variables (`VITE_API_URL`, `VITE_WS_URL`) để cấu hình API endpoint
- Material-UI cung cấp components và theme
- Zustand quản lý global state (alerts, flows, rules)
- WebSocket tự động reconnect khi mất kết nối
- REST API được sử dụng cho initial load và pagination
- WebSocket được sử dụng cho real-time updates (alerts, flows)

### 3.2.2.4. Hubble Client (hubble_client.go)

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

### 3.2.2.5. Metrics Collector (metrics_collector.go)

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

### 3.2.2.6. Rule Engine (engine.go)

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
    Start(ctx context.Context)
}
```

**Giải thích:**
- Sử dụng interface để dễ dàng thêm rules mới
- Thread-safe với mutex để bảo vệ shared state
- Alert channel để decouple rule evaluation và alert delivery

### 3.2.2.7. Tương tác giữa UI và API Server

**Kiến trúc giao tiếp:**

```
┌─────────────┐                    ┌──────────────┐
│     UI      │                    │  API Server  │
│  (React)    │                    │    (Go)     │
└──────┬──────┘                    └──────┬───────┘
       │                                  │
       │ 1. REST API (Initial Load)      │
       ├─────────────────────────────────►│
       │  GET /api/v1/flows              │
       │  GET /api/v1/alerts              │
       │  GET /api/v1/rules               │
       │                                  │
       │ 2. WebSocket (Real-time)        │
       ├─────────────────────────────────►│
       │  WS /api/v1/stream/alerts       │
       │  WS /api/v1/stream/flows        │
       │                                  │
       │ 3. REST API (Actions)           │
       ├─────────────────────────────────►│
       │  PUT /api/v1/rules/{id}         │
       │                                  │
       │◄─────────────────────────────────┤
       │  Response (JSON)                │
       │  WebSocket Messages (JSON)      │
       │                                  │
```

**Các pattern giao tiếp:**

1. **Initial Data Load (REST API):**
   - UI gọi REST API khi component mount để lấy dữ liệu ban đầu
   - Sử dụng pagination để giảm tải
   - Có thể filter và search

2. **Real-time Updates (WebSocket):**
   - UI kết nối WebSocket sau khi load dữ liệu ban đầu
   - Nhận alerts và flows mới real-time
   - Tự động update UI mà không cần refresh

3. **User Actions (REST API):**
   - Khi user thực hiện action (enable/disable rule, filter, search)
   - UI gọi REST API với parameters tương ứng
   - API Server trả về kết quả filtered

**Code example - Dashboard component:**

```javascript
// Dashboard.jsx
function Dashboard() {
  const [stats, setStats] = useState({})
  const [alerts, setAlerts] = useState([])
  
  // 1. Initial load với REST API
  useEffect(() => {
    const loadData = async () => {
      // Load stats
      const statsRes = await metricsAPI.getPrometheusStats()
      setStats(statsRes.data)
      
      // Load alerts
      const alertsRes = await alertsAPI.getAll({ limit: 50 })
      setAlerts(alertsRes.data)
    }
    loadData()
  }, [])
  
  // 2. Real-time updates với WebSocket
  useEffect(() => {
    const ws = createWebSocket('/stream/alerts')
    
    ws.onmessage = (event) => {
      const alert = JSON.parse(event.data)
      // Thêm alert mới vào đầu danh sách
      setAlerts(prev => [alert, ...prev].slice(0, 50))
    }
    
    return () => ws.close()
  }, [])
  
  // 3. User actions
  const handleFilter = async (severity) => {
    const res = await alertsAPI.getAll({ severity, limit: 50 })
    setAlerts(res.data)
  }
  
  return (
    // UI components
  )
}
```

**Code example - API Server handlers:**

```go
// handlers/handlers.go

// REST API handler
func (h *Handlers) GetAlerts(w http.ResponseWriter, r *http.Request) {
    severity := r.URL.Query().Get("severity")
    limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
    
    alerts := h.store.GetAlerts(limit, severity, "", "")
    json.NewEncoder(w).Encode(alerts)
}

// WebSocket handler
func (h *Handlers) StreamAlerts(w http.ResponseWriter, r *http.Request) {
    conn, _ := upgrader.Upgrade(w, r, nil)
    defer conn.Close()
    
    // Subscribe to alerts
    sub := &storage.AlertSubscriber{
        Channel: make(chan storage.Alert),
        Filter: storage.AlertFilter{
            Severity: r.URL.Query().Get("severity"),
        },
    }
    h.store.SubscribeAlerts(sub)
    defer h.store.UnsubscribeAlerts(sub)
    
    // Send alerts to client
    for alert := range sub.Channel {
        conn.WriteJSON(alert)
    }
}
```

**Giải thích:**
- **REST API**: Dùng cho initial load, pagination, filtering, user actions
- **WebSocket**: Dùng cho real-time updates, giảm số lượng HTTP requests
- **Hybrid approach**: Kết hợp REST API và WebSocket để tối ưu performance
- **State management**: UI quản lý state local, WebSocket chỉ update khi có dữ liệu mới
- **Error handling**: UI có retry logic và fallback khi API/WebSocket lỗi

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

### 3.2.3.2. Rule 1: DDoS Detection (Real-time)

**File implementation:** `internal/rules/builtin/ddos_rule.go`

**Mục đích:** Phát hiện tấn công DDoS real-time bằng cách theo dõi trực tiếp từ flows

**Cơ chế hoạt động:**

1. **Real-time Flow Monitoring:**
   - Theo dõi trực tiếp từ flows qua method `Evaluate()`
   - Đếm số lượng flows trong time window (1 phút)
   - Không cần query Prometheus, phát hiện ngay lập tức

2. **Baseline Collection (5 phút đầu):**
   - Thu thập flows trong 5 phút để tính baseline
   - Tính rate: `baseline = total_flows / 5_minutes`

3. **Anomaly Detection:**
   - So sánh current rate với baseline mỗi phút
   - Alert nếu: `current_rate > baseline * threshold`

**Cấu hình:**

```yaml
rules:
  - name: "ddos"
    enabled: true
    severity: "CRITICAL"
    description: "Phát hiện DDoS attack real-time"
    thresholds:
      multiplier: 3.0  # Alert nếu traffic > 3x baseline
```

**Code quan trọng:**

```go
func (r *DDoSRule) Evaluate(ctx context.Context, flow *model.Flow) *model.Alert {
    if !r.enabled || flow == nil {
        return nil
    }
    
    namespace := "unknown"
    if flow.Source != nil && flow.Source.Namespace != "" {
        namespace = flow.Source.Namespace
    }
    
    // Đếm flows trong window
    r.flowCounts[namespace]++
    
    // Check nếu window đã hết (1 phút)
    if elapsed >= r.window {
        alert := r.checkDDoSAttack(namespace, elapsed)
        r.flowCounts[namespace] = 0
        return alert
    }
    return nil
}

func (r *DDoSRule) checkDDoSAttack(namespace string, elapsed time.Duration) *model.Alert {
    baseline := r.baseline[namespace]
    currentRate := float64(r.flowCounts[namespace]) / elapsed.Minutes()
    multiplier := currentRate / baseline
    
    if multiplier > r.threshold {
        alert := &model.Alert{
            Type:      "ddos",
            Severity:  r.severity,
            Namespace: namespace,
            Message:   fmt.Sprintf("DDoS attack detected: %.2fx baseline", multiplier),
        }
        return alert
    }
    return nil
}
```

**Ưu điểm:**
- Phát hiện real-time, không có độ trễ từ Prometheus
- Tự động học baseline, không cần cấu hình thủ công
- Theo dõi theo từng namespace riêng biệt
- Hiệu quả hơn so với query Prometheus

### 3.2.3.3. Rule 2: Traffic Spike Detection (Prometheus-based)

**File implementation:** `internal/rules/builtin/traffic_spike_rule.go`

**Mục đích:** Phát hiện tấn công DDoS dựa trên sự tăng đột biến traffic từ Prometheus metrics

**Cơ chế hoạt động:**

1. **Baseline Collection (1 phút đầu):**
   - Query Prometheus: `rate(hubble_flows_total{namespace="X"}[1m])`
   - Thu thập traffic rate trong 1 phút để tính baseline
   - Tính trung bình các samples trong window

2. **Anomaly Detection (mỗi 10 giây):**
   - Query Prometheus định kỳ: `rate(hubble_flows_total{namespace="X"}[1m])`
   - So sánh current rate với baseline
   - Alert nếu: `current_rate > baseline * threshold`

**Cấu hình:**

```yaml
rules:
  - name: "traffic_spike"
    enabled: true
    severity: "CRITICAL"
    description: "Phát hiện traffic spike: lưu lượng tăng nhanh trong thời gian ngắn"
    thresholds:
      multiplier: 3.0  # Alert nếu traffic > 3x baseline
```

**Code quan trọng:**

```go
func (r *TrafficSpikeRule) Start(ctx context.Context) {
    ticker := time.NewTicker(r.interval)  // 10 giây
    defer ticker.Stop()
    
    for {
        select {
        case <-ticker.C:
            r.checkFromPrometheus(ctx)
        case <-ctx.Done():
            return
        }
    }
}

func (r *TrafficSpikeRule) checkNamespace(ctx context.Context, namespace string) {
    query := fmt.Sprintf(`rate(hubble_flows_total{namespace="%s"}[1m])`, namespace)
    result, err := r.prometheusAPI.Query(ctx, query, 10*time.Second)
    
    currentRate := float64(vector[0].Value)
    baseline := r.baseline[namespace]
    multiplier := currentRate / baseline
    
    if multiplier > r.threshold {
        alert := &model.Alert{
            Type:      "traffic_spike",
            Severity:  r.severity,
            Namespace: namespace,
            Message:   fmt.Sprintf("Traffic spike: %.2fx baseline", multiplier),
        }
        r.alertEmitter(alert)
    }
}
```

**Ưu điểm:**
- Sử dụng Prometheus metrics có sẵn
- Có thể query historical data
- Tự động học baseline
- Có thể điều chỉnh sensitivity qua threshold

**So sánh với DDoS Rule:**
- **DDoS Rule**: Real-time, nhanh hơn, theo dõi trực tiếp từ flows
- **Traffic Spike Rule**: Dựa trên Prometheus, có độ trễ nhưng có thể query historical data

### 3.2.3.4. Rule 3: Port Scan Detection

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

### 3.2.3.5. Rule 4: Block Connection Detection

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

### 3.2.3.6. Rule 5: Namespace Access Detection

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

### 3.2.3.7. Rule 6: Suspicious Outbound Detection

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
    ├──► Anomaly Detector (cmd/hubble-guard)
    │    │
    │    │ (Process Flows)
    │    ├──► Rule Engine
    │    │    │
    │    │    │ (Evaluate Rules)
    │    │    ├──► Alert Generation
    │    │    │    │
    │    │    │    ├──► Telegram Notifier
    │    │    │    ├──► Log Notifier
    │    │    │    └──► Webhook Notifier
    │    │    │
    │    │    └──► Prometheus Metrics
    │    │
    │    └──► Prometheus Server
    │
    └──► API Server (api/main.go)
         │
         │ (Stream Flows)
         ├──► In-Memory Storage
         │    │
         │    ├──► Flows (max 50k)
         │    ├──► Alerts (max 10k)
         │    └──► Rules
         │
         │ (REST API / WebSocket)
         └──► UI (React)
              │
              │ (Display)
              └──► Web Browser
```

### 3.2.4.2. Chi tiết các bước xử lý

**Bước 1: Thu thập dữ liệu từ Hubble**

```go
// Anomaly Detector stream flows từ Hubble Relay
stream, err := detectorClient.GetFlows(ctx, req)
for {
    response, err := stream.Recv()
    flow := convertHubbleFlow(response.GetFlow())
    // Xử lý flow trong detector
}

// API Server cũng stream flows riêng cho UI
stream, err := apiClient.GetFlows(ctx, req)
for {
    response, err := stream.Recv()
    flow := convertHubbleFlow(response.GetFlow())
    store.AddFlow(flow)  // Lưu vào storage
    broadcastToWebSocketClients(flow)  // Gửi đến UI
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

**Bước 5: UI hiển thị dữ liệu**

```javascript
// UI gọi REST API để lấy dữ liệu
const flows = await flowsAPI.getAll({ page: 1, limit: 25 })
const alerts = await alertsAPI.getAll({ severity: 'CRITICAL' })

// UI kết nối WebSocket để nhận real-time updates
const ws = createWebSocket('/stream/alerts')
ws.onmessage = (event) => {
  const alert = JSON.parse(event.data)
  // Update UI với alert mới
  updateAlerts(alert)
}
```

**Giải thích:**
- **Hai luồng độc lập**: Anomaly Detector và API Server đều stream từ Hubble riêng biệt
- **Detector**: Xử lý flows, đánh giá rules, phát sinh alerts
- **API Server**: Lưu flows vào storage, cung cấp API cho UI
- **UI**: Gọi REST API để lấy dữ liệu, dùng WebSocket để nhận real-time updates
- **Decoupling**: UI không phụ thuộc trực tiếp vào detector, chỉ giao tiếp với API Server

---

## 3.2.5. Các điểm kỹ thuật quan trọng

### 3.2.5.1. Thread Safety

- Sử dụng `sync.RWMutex` để bảo vệ shared state
- Rule Engine sử dụng mutex khi đăng ký rules
- Metrics collector thread-safe nhờ Prometheus client
- API Server storage sử dụng mutex cho concurrent access

### 3.2.5.2. Error Handling

- Graceful degradation: Nếu Prometheus không available, rules vẫn chạy nhưng không query
- Retry mechanism cho Prometheus queries
- Logging chi tiết để debug
- API Server có health check endpoint (`/health`)

### 3.2.5.3. Performance Optimization

- gRPC streaming thay vì polling
- Time-windowed tracking cho port scan (tránh memory leak)
- Batch processing cho metrics
- WebSocket cho real-time updates thay vì polling
- In-memory storage với giới hạn (50k flows, 10k alerts)

### 3.2.5.4. Configuration Management

- YAML-based configuration
- Environment variable support cho UI (VITE_API_URL, VITE_WS_URL)
- Default values cho tất cả settings
- Validation khi load config

### 3.2.5.5. Separation of Concerns

- **Detector**: Chỉ xử lý flows và phát hiện anomalies
- **API Server**: Chỉ cung cấp API và storage cho UI
- **UI**: Chỉ hiển thị dữ liệu, không xử lý logic
- Mỗi component có thể scale độc lập

---

## Phụ lục: Template cho phần 3.2

### 3.2.1. Kiến trúc tổng quan hệ thống
- [ ] Mô hình kiến trúc (diagram)
- [ ] Cấu trúc thư mục dự án
- [ ] Giải thích lựa chọn kiến trúc

### 3.2.2. Các thành phần chính
- [ ] Entry Point (main.go)
- [ ] API Server (api/main.go)
- [ ] UI (Web Interface)
- [ ] Hubble Client
- [ ] Metrics Collector
- [ ] Rule Engine
- [ ] Alert System

### 3.2.3. Cấu hình Rules
- [ ] Cấu trúc file cấu hình
- [ ] Rule 1: DDoS Detection (Real-time)
- [ ] Rule 2: Traffic Spike (Prometheus-based)
- [ ] Rule 3: Port Scan
- [ ] Rule 4: Block Connection
- [ ] Rule 5: Namespace Access
- [ ] Rule 6: Suspicious Outbound

### 3.2.4. Luồng xử lý dữ liệu
- [ ] Data Flow Diagram
- [ ] Chi tiết các bước xử lý
- [ ] Timing và synchronization
- [ ] Luồng dữ liệu từ Detector đến UI

### 3.2.5. Điểm kỹ thuật quan trọng
- [ ] Thread Safety
- [ ] Error Handling
- [ ] Performance Optimization
- [ ] Configuration Management
- [ ] Separation of Concerns


