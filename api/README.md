# Hubble Anomaly Detector - API Server

HTTP API server cho UI frontend. Server này cung cấp REST API endpoints và WebSocket để UI có thể hiển thị và quản lý flows, alerts, và rules.

## Kiến trúc

- **Ngôn ngữ**: Go
- **Framework**: Gorilla Mux (HTTP router)
- **Storage**: In-memory (không cần database)
- **WebSocket**: Gorilla WebSocket cho real-time alerts
- **Port**: 5001 (mặc định)

## Cấu trúc

```
api/
├── main.go                    # Entry point
├── internal/
│   ├── handlers/              # HTTP handlers
│   │   └── handlers.go
│   └── storage/               # In-memory storage
│       └── storage.go
└── README.md
```

## Tính năng

### 1. Flows API
- `GET /api/v1/flows` - Lấy danh sách flows với pagination và filter
- `GET /api/v1/flows/:id` - Chi tiết flow
- `GET /api/v1/flows/stats` - Thống kê flows

### 2. Alerts API
- `GET /api/v1/alerts` - Lấy danh sách alerts với filter
- `GET /api/v1/alerts/:id` - Chi tiết alert
- `GET /api/v1/alerts/timeline` - Timeline alerts
- `WS /api/v1/stream/alerts` - WebSocket stream alerts real-time

### 3. Rules API
- `GET /api/v1/rules` - Lấy danh sách rules
- `GET /api/v1/rules/:id` - Chi tiết rule
- `PUT /api/v1/rules/:id` - Cập nhật rule (enable/disable, severity, description)
- `GET /api/v1/rules/stats` - Thống kê rules

### 4. Metrics API
- `GET /api/v1/metrics/stats` - Metrics tổng quan
- `GET /api/v1/metrics/prometheus` - Query Prometheus

## Cài đặt

### Prerequisites

- Go 1.24+
- Access đến config file `configs/anomaly_detection.yaml`
- Prometheus (optional, cho metrics queries)

### Install Dependencies

```bash
go mod tidy
```
### Chạy Server

**Cách 1: Từ root directory (Khuyến nghị)**

```bash
# Từ root directory của project
make api-run
```

Hoặc chạy trực tiếp:
```bash
go run api/main.go -port=5001 -config=configs/anomaly_detection.yaml
```

**Cách 2: Từ thư mục api**

```bash
cd api
make run
```

Hoặc với custom port:
```bash
make run API_PORT=5002
```

**Xem tất cả lệnh Makefile:**
```bash
# Từ root
make help

# Hoặc từ api/
cd api && make help
```

### Build

**Sử dụng Makefile:**
```bash
cd api
make build
```

**Hoặc build trực tiếp:**
```bash
go build -o bin/api-server api/main.go
./bin/api-server
```

## Cấu hình

API server đọc config từ file YAML (giống core app):
- `configs/anomaly_detection.yaml`

Config được dùng để:
- Kết nối Prometheus (cho metrics queries)
- Sync rules từ config file (mỗi 30 giây)

## Storage

API server sử dụng **in-memory storage**:
- **Alerts**: Lưu tối đa 10,000 alerts gần nhất
- **Flows**: Lưu tối đa 50,000 flows gần nhất
- **Rules**: Sync từ config file

**Lưu ý**: Data sẽ mất khi server restart. Để persist data, cần:
- Tích hợp với core app để nhận data
- Hoặc thêm database (PostgreSQL, MongoDB, etc.)

## Tích hợp với Core App

Hiện tại API server chạy độc lập. Để nhận data từ core app, có 2 cách:

### Cách 1: Core app gửi data đến API server (Recommended)

Thêm vào core app (`cmd/hubble-detector/main.go`):
```go
// Gửi alert đến API server
func sendAlertToAPI(alert model.Alert) {
    // POST to http://localhost:5001/api/v1/alerts
}
```

### Cách 2: API server query từ Prometheus

API server đã có sẵn Prometheus client để query metrics.

## API Endpoints

### Flows

```
GET /api/v1/flows?page=1&limit=25&namespace=default&verdict=FORWARDED&search=pod
GET /api/v1/flows/{id}
GET /api/v1/flows/stats
```

### Alerts

```
GET /api/v1/alerts?limit=50&severity=CRITICAL&namespace=default&search=ddos
GET /api/v1/alerts/{id}
GET /api/v1/alerts/timeline?start=2024-01-01T00:00:00Z&end=2024-01-01T23:59:59Z
WS  /api/v1/stream/alerts?severity=CRITICAL&namespace=default
```

### Rules

```
GET /api/v1/rules
GET /api/v1/rules/{id}
PUT /api/v1/rules/{id}
Body: {"enabled": true, "severity": "HIGH", "description": "..."}
GET /api/v1/rules/stats
```

### Metrics

```
GET /api/v1/metrics/stats
GET /api/v1/metrics/prometheus?query=rate(hubble_flows_total[5m])
```

## CORS

API server đã enable CORS để cho phép UI gọi từ domain khác.

## WebSocket

WebSocket endpoint `/api/v1/stream/alerts`:
- Hỗ trợ filter: `?severity=CRITICAL&namespace=default&type=ddos`
- Gửi alerts real-time khi có alert mới
- Auto ping/pong để keep connection alive

## Health Check

```
GET /health
Response: "OK"
```

## Lưu ý

1. **Không ảnh hưởng Core App**: API server chạy độc lập, không modify core app
2. **In-memory Storage**: Data mất khi restart, cần tích hợp với core app để persist
3. **Rules Sync**: Rules được sync từ config file mỗi 30 giây
4. **Prometheus**: Cần Prometheus running để query metrics

## Development

### Test API

```bash
# Health check
curl http://localhost:5001/health

# Get flows
curl http://localhost:5001/api/v1/flows

# Get alerts
curl http://localhost:5001/api/v1/alerts

# Get rules
curl http://localhost:5001/api/v1/rules
```

### Test WebSocket

Sử dụng tool như `wscat`:
```bash
wscat -c ws://localhost:5001/api/v1/stream/alerts?severity=CRITICAL
```

## Troubleshooting

### Port đã được sử dụng
```bash
# Đổi port
go run api/main.go -port=5002
```

### Prometheus connection failed
- Kiểm tra Prometheus URL trong config
- Đảm bảo Prometheus đang chạy

### Rules không sync
- Kiểm tra config file path
- Xem logs để biết lỗi

