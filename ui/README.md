# Hubble Anomaly Detector - UI

Giao diện người dùng cho hệ thống phát hiện bất thường mạng Hubble.

## Công nghệ

- **Framework**: React 18+ với JavaScript
- **Build tool**: Vite 
- **UI Library**: Material-UI (MUI)
- **State Management**: Zustand
- **API Client**: Axios
- **Charts**: Recharts
- **Routing**: React Router

## Cấu trúc thư mục

```
ui/
├── src/
│   ├── components/        # React components
│   │   └── Layout.jsx     # Main layout với navigation
│   ├── pages/             # Pages/routes
│   │   ├── Dashboard.jsx
│   │   ├── AnomalyDetection.jsx
│   │   ├── FlowViewer.jsx
│   │   └── RulesManagement.jsx
│   ├── services/          # API services
│   │   └── api.js
│   ├── store/             # Zustand store
│   │   └── useStore.js
│   ├── theme.js           # MUI theme configuration
│   ├── App.jsx            # Main App component
│   └── main.jsx           # Entry point
├── package.json
├── vite.config.js
├── Dockerfile
├── nginx.conf
└── README.md
```

## Tính năng

### 1. Dashboard
- Tổng quan flows real-time
- Biểu đồ traffic theo thời gian
- Thống kê flows by namespace, verdict, port
- Cards hiển thị metrics tổng quan

### 2. Anomaly Detection
- Danh sách alerts real-time
- Filter theo severity (CRITICAL, HIGH, MEDIUM, LOW)
- Chi tiết từng alert
- WebSocket để nhận alerts real-time

### 3. Flow Viewer
- Bảng flows với pagination
- Filter flows theo namespace, pod, port
- Search flows
- Export flows to CSV

### 4. Rules Management
- Danh sách rules đang active
- Enable/disable rules
- Cấu hình thresholds
- Edit rule settings

## Development

### Prerequisites

- Node.js 18+
- npm hoặc yarn

### Setup

```bash
cd ui
npm install
```

### Development Server

```bash
npm run dev
```

UI sẽ chạy tại `http://localhost:5000`

### Build

```bash
npm run build
```

Output sẽ ở trong thư mục `dist/`

## Environment Variables

Tạo file `.env` trong thư mục `ui/`:

```env
VITE_API_URL=http://localhost:5001/api/v1
VITE_WS_URL=ws://localhost:5001/api/v1
```

## Docker Build

```bash
# Build image
docker build -t hubble-ui:1.0.0 .

# Run container
docker run -p 5000:80 hubble-ui:1.0.0
```

## Deployment với Helm

UI đã được tích hợp vào Helm chart. Để deploy:

```bash
# Build và push UI image
docker build -t your-registry/hubble-ui:1.0.0 ./ui
docker push your-registry/hubble-ui:1.0.0

# Deploy với Helm
helm upgrade --install hubble-guard ./helm/hubble-guard \
  -n hubble-guard \
  --create-namespace \
  --set ui.enabled=true \
  --set ui.image.repository=your-registry/hubble-ui \
  --set ui.image.tag=1.0.0
```

## API Endpoints cần thiết

Backend cần expose các REST API endpoints:

```
GET  /api/v1/flows              # Lấy danh sách flows
GET  /api/v1/flows/:id          # Chi tiết flow
GET  /api/v1/flows/stats        # Thống kê flows
GET  /api/v1/alerts             # Lấy danh sách alerts
GET  /api/v1/alerts/:id         # Chi tiết alert
GET  /api/v1/alerts/timeline    # Timeline alerts
GET  /api/v1/rules              # Lấy danh sách rules
GET  /api/v1/rules/:id          # Chi tiết rule
PUT  /api/v1/rules/:id          # Cập nhật rule
GET  /api/v1/rules/stats        # Thống kê rules
GET  /api/v1/metrics/stats      # Metrics tổng quan
WS   /api/v1/stream/alerts      # WebSocket stream alerts
```

## Notes

- UI sẽ giao tiếp với backend qua REST API
- Real-time updates sử dụng WebSocket
- Nginx được cấu hình để proxy API requests đến backend
- CORS cần được config ở backend nếu deploy riêng
