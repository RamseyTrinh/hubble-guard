# 3.3. Thiết kế và triển khai giao diện người dùng

## 3.3.1. Tổng quan

Giao diện người dùng (UI) của hệ thống Hubble Guard được xây dựng như một ứng dụng web độc lập, cung cấp khả năng giám sát và quản lý mạng Kubernetes theo thời gian thực. UI được thiết kế với kiến trúc Single Page Application (SPA), cho phép người dùng tương tác mượt mà mà không cần tải lại trang.

### 3.3.1.1. Mục tiêu thiết kế

- **Trực quan hóa dữ liệu**: Hiển thị thông tin mạng, flows, và alerts một cách trực quan, dễ hiểu
- **Real-time monitoring**: Cập nhật dữ liệu theo thời gian thực thông qua WebSocket
- **Responsive design**: Tương thích với nhiều kích thước màn hình khác nhau
- **Dark theme**: Giao diện tối giúp giảm mỏi mắt khi làm việc lâu dài
- **Hiệu suất cao**: Tối ưu hóa tốc độ tải và phản hồi

### 3.3.1.2. Kiến trúc tổng thể

UI hoạt động như một lớp trình bày độc lập, giao tiếp với API Server thông qua:
- **REST API**: Cho các thao tác truy vấn dữ liệu (GET, POST, PUT)
- **WebSocket**: Cho việc nhận dữ liệu real-time (flows, alerts)
- **HTTP Proxy**: Nginx được sử dụng để phục vụ static files và proxy requests

```
┌─────────────────┐
│   Web Browser   │
└────────┬────────┘
         │ HTTP/WebSocket
         ▼
┌─────────────────┐
│   UI (React)    │ ◄─── Nginx (Static Files)
│   Port: 80      │
└────────┬────────┘
         │ REST API / WebSocket
         ▼
┌─────────────────┐
│  API Server     │
│  Port: 5001     │
└─────────────────┘
```

## 3.3.2. Công nghệ và công cụ

### 3.3.2.1. Frontend Framework

**React 18.2+**: Framework JavaScript phổ biến cho việc xây dựng giao diện người dùng
- Component-based architecture: Tái sử dụng code, dễ bảo trì
- Virtual DOM: Tối ưu hiệu suất rendering
- Hooks API: Quản lý state và side effects hiệu quả

### 3.3.2.2. Build Tool

**Vite 5.0+**: Build tool hiện đại, nhanh chóng
- Hot Module Replacement (HMR): Tải lại nhanh khi development
- ES modules: Hỗ trợ native ES modules
- Optimized production builds: Tối ưu hóa tự động cho production

### 3.3.2.3. UI Library

**Material-UI (MUI) 5.14+**: Component library dựa trên Material Design
- Rich component set: Buttons, Cards, Tables, Charts, etc.
- Customizable theme: Dễ dàng tùy chỉnh màu sắc, typography
- Responsive grid system: Layout linh hoạt
- Dark mode support: Hỗ trợ sẵn dark theme

### 3.3.2.4. State Management

**Zustand 4.4+**: Lightweight state management library
- Simple API: Dễ sử dụng, không cần boilerplate code
- Small bundle size: Chỉ ~1KB gzipped
- TypeScript support: Type-safe state management

### 3.3.2.5. Data Visualization

**Recharts 2.10+**: Library vẽ biểu đồ dựa trên D3.js
- Line charts: Hiển thị time-series data
- Pie charts: Phân bố dữ liệu theo loại
- Responsive: Tự động điều chỉnh theo kích thước màn hình

### 3.3.2.6. HTTP Client

**Axios 1.6+**: Promise-based HTTP client
- Interceptors: Xử lý requests/responses tập trung
- Request/Response transformation: Chuyển đổi dữ liệu tự động
- Error handling: Xử lý lỗi nhất quán

### 3.3.2.7. Routing

**React Router 6.20+**: Client-side routing
- Declarative routing: Định nghĩa routes dễ dàng
- Nested routes: Hỗ trợ nested routing
- Navigation guards: Kiểm soát truy cập routes

## 3.3.3. Cấu trúc dự án

### 3.3.3.1. Cấu trúc thư mục

```
ui/
├── src/
│   ├── components/           # Reusable components
│   │   ├── Layout.jsx        # Main layout với navigation drawer
│   │   └── GrafanaEmbed.jsx  # Component embed Grafana dashboard
│   ├── pages/                # Page components (routes)
│   │   ├── Dashboard.jsx     # Trang tổng quan
│   │   └── FlowViewer.jsx    # Trang xem flows
│   ├── services/             # API services
│   │   └── api.js            # Axios client và API endpoints
│   ├── store/                # State management
│   │   └── useStore.js       # Zustand store definition
│   ├── theme.js              # MUI theme configuration
│   ├── App.jsx               # Root component với routing
│   └── main.jsx              # Entry point
├── public/                   # Static assets
├── package.json              # Dependencies và scripts
├── vite.config.js           # Vite configuration
├── Dockerfile               # Docker build instructions
├── nginx.conf               # Nginx configuration
└── README.md                # Documentation
```

### 3.3.3.2. Mô tả các module chính

**1. Entry Point (`main.jsx`)**
```javascript
import React from 'react'
import ReactDOM from 'react-dom/client'
import { BrowserRouter } from 'react-router-dom'
import { ThemeProvider } from '@mui/material/styles'
import CssBaseline from '@mui/material/CssBaseline'
import App from './App'
import theme from './theme'

ReactDOM.createRoot(document.getElementById('root')).render(
  <React.StrictMode>
    <BrowserRouter>
      <ThemeProvider theme={theme}>
        <CssBaseline />
        <App />
      </ThemeProvider>
    </BrowserRouter>
  </React.StrictMode>
)
```

**2. Root Component (`App.jsx`)**
- Định nghĩa routes và navigation structure
- Wrap tất cả pages trong Layout component

**3. Layout Component (`Layout.jsx`)**
- Responsive navigation drawer
- AppBar với menu toggle
- Sidebar navigation với các menu items

**4. API Service (`services/api.js`)**
- Cấu hình Axios instance với base URL
- Định nghĩa các API endpoints (flows, alerts, rules, metrics)
- WebSocket helper functions

**5. State Store (`store/useStore.js`)**
- Centralized state cho flows, alerts, rules
- Actions để update state
- Loading và error states

## 3.3.4. Các trang chính

### 3.3.4.1. Dashboard (Trang tổng quan)

Dashboard cung cấp cái nhìn tổng quan về trạng thái hệ thống, bao gồm:

**a) Statistics Cards**
- **Total Flows**: Tổng số flows đã xử lý
- **Total Alerts**: Tổng số alerts đã phát hiện
- **Critical Alerts**: Số lượng alerts mức độ nghiêm trọng
- **TCP Connections**: Số lượng kết nối TCP hiện tại

Mỗi card hiển thị:
- Icon đại diện
- Giá trị số với format dễ đọc (toLocaleString)
- Loading state khi đang tải dữ liệu
- Hover effect để tăng tương tác

**b) Dropped Flows Time-Series Chart**
- Biểu đồ đường (Line Chart) hiển thị số lượng dropped flows theo thời gian
- Time range: 1 giờ gần nhất
- Data points: Mỗi 15 giây
- Tự động refresh mỗi 30 giây

**c) Alert Types Pie Chart**
- Biểu đồ tròn phân bố alerts theo loại
- Màu sắc phân biệt cho từng loại alert
- Tooltip hiển thị số lượng chi tiết

**d) Grafana Dashboard Embed**
- Embed Grafana dashboard trực tiếp vào UI
- Kiosk mode để ẩn navigation
- Auto-refresh mỗi 30 giây
- Có thể mở trong tab mới

**Code snippet - StatCard Component:**
```javascript
function StatCard({ title, value, icon: Icon, color, loading }) {
  return (
    <Card sx={{ 
      height: "100%",
      transition: "transform 0.2s, box-shadow 0.2s",
      "&:hover": {
        transform: "translateY(-4px)",
        boxShadow: 4,
      },
    }}>
      <CardContent>
        <Box display="flex" alignItems="center" justifyContent="space-between">
          <Box sx={{ p: 1.5, borderRadius: 2, bgcolor: colors.light }}>
            <Icon />
          </Box>
          {loading ? (
            <CircularProgress size={24} />
          ) : (
            <Typography variant="h4" fontWeight="bold">
              {typeof value === "number" ? value.toLocaleString() : value}
            </Typography>
          )}
        </Box>
        <Typography variant="h6" color="text.secondary">
          {title}
        </Typography>
      </CardContent>
    </Card>
  )
}
```

### 3.3.4.2. Flow Viewer (Trang xem flows)

Flow Viewer cho phép người dùng xem, tìm kiếm và lọc các network flows:

**a) Tính năng chính:**
- **Pagination**: Phân trang với 25 flows mỗi trang
- **Search**: Tìm kiếm flows theo source/destination IP, pod name
- **Filters**: 
  - Filter theo namespace
  - Filter theo verdict (FORWARDED, DROPPED, ERROR)
- **Export**: Xuất flows ra file CSV
- **Real-time updates**: Tự động cập nhật khi có flow mới (chỉ khi ở trang đầu)

**b) Bảng flows hiển thị:**
- Source Pod, Source IP, Source Identity
- Destination Pod, Destination IP, Destination Identity
- Destination Port
- Traffic Direction (INGRESS/EGRESS)
- Verdict (với màu sắc phân biệt):
  - FORWARDED: Xanh lá
  - DROPPED: Đỏ
  - TRACED: Xanh dương
  - TRANSLATED: Vàng
- TCP Flags
- Timestamp (formatted)

**c) WebSocket Integration:**
```javascript
useEffect(() => {
  const wsUrl = "ws://localhost:5001/api/v1/stream/flows"
  const ws = new WebSocket(wsUrl)
  
  ws.onmessage = () => {
    // Silent refresh: chỉ refresh nếu đang ở trang đầu
    if (page === 0) {
      loadFlows(true) // true = silent refresh (không hiện loading)
    }
  }
  
  return () => ws.close()
}, [page])
```

**d) Export to CSV:**
```javascript
const handleExport = () => {
  const csv = [
    ["Source Pod", "Source IP", ...].join(","),
    ...flows.map((f) => [
      f.source?.name || "",
      f.source_ip || "",
      // ... other fields
    ].join(","))
  ].join("\n")
  
  const blob = new Blob([csv], { type: "text/csv" })
  const url = window.URL.createObjectURL(blob)
  const a = document.createElement("a")
  a.href = url
  a.download = `flows-${new Date().toISOString()}.csv`
  a.click()
}
```

### 3.3.4.3. Layout và Navigation

Layout component cung cấp cấu trúc chung cho toàn bộ ứng dụng:

**a) Responsive Design:**
- **Desktop**: Permanent drawer (sidebar luôn hiển thị)
- **Mobile**: Temporary drawer (mở/đóng bằng menu button)

**b) Navigation Items:**
- Dashboard: `/dashboard` hoặc `/`
- Flow Viewer: `/flows`

**c) AppBar:**
- Hiển thị title "Hubble Guard Monitoring"
- Menu toggle button (chỉ hiển thị trên mobile)

**Code snippet - Layout Component:**
```javascript
export default function Layout({ children }) {
  const [mobileOpen, setMobileOpen] = useState(false)
  const navigate = useNavigate()
  const location = useLocation()

  return (
    <Box sx={{ display: 'flex' }}>
      <AppBar position="fixed" sx={{ width: { sm: `calc(100% - ${drawerWidth}px)` } }}>
        <Toolbar>
          <IconButton onClick={handleDrawerToggle} sx={{ display: { sm: 'none' } }}>
            <MenuIcon />
          </IconButton>
          <Typography variant="h6">Hubble Guard Monitoring</Typography>
        </Toolbar>
      </AppBar>
      
      <Drawer variant="permanent" sx={{ display: { xs: 'none', sm: 'block' } }}>
        {/* Navigation items */}
      </Drawer>
      
      <Box component="main" sx={{ flexGrow: 1, p: 3 }}>
        <Toolbar />
        {children}
      </Box>
    </Box>
  )
}
```

## 3.3.5. State Management

### 3.3.5.1. Zustand Store Structure

Store được tổ chức theo các domain chính:

```javascript
const useStore = create((set) => ({
  // Flows state
  flows: [],
  totalFlows: 0,
  flowsLoading: false,
  flowsError: null,
  
  // Alerts state
  alerts: [],
  alertsLoading: false,
  alertsError: null,
  
  // Rules state
  rules: [],
  rulesLoading: false,
  rulesError: null,
  
  // Stats state
  stats: null,
  statsLoading: false,
  
  // Actions
  setFlows: (flows) => set({ flows }),
  setFlowsLoading: (loading) => set({ flowsLoading: loading }),
  // ... other actions
}))
```

### 3.3.5.2. Sử dụng Store trong Components

```javascript
import useStore from '../store/useStore'

function FlowViewer() {
  const { 
    flows, 
    setFlows, 
    flowsLoading, 
    setFlowsLoading 
  } = useStore()
  
  // Component logic...
}
```

## 3.3.6. Tích hợp với API Server

### 3.3.6.1. REST API Integration

**API Client Configuration:**
```javascript
const API_BASE_URL = import.meta.env.VITE_API_URL || 'http://localhost:5001/api/v1'

const api = axios.create({
  baseURL: API_BASE_URL,
  timeout: 30000,
  headers: {
    'Content-Type': 'application/json',
  },
})
```

**API Endpoints:**
```javascript
export const flowsAPI = {
  getAll: (params = {}) => api.get('/flows', { params }),
  getById: (id) => api.get(`/flows/${id}`),
  getStats: () => api.get('/flows/stats'),
}

export const alertsAPI = {
  getAll: (params = {}) => api.get('/alerts', { params }),
  getById: (id) => api.get(`/alerts/${id}`),
  getTimeline: (params = {}) => api.get('/alerts/timeline', { params }),
}

export const metricsAPI = {
  getPrometheusStats: () => api.get('/metrics/prometheus/stats'),
  getDroppedFlowsTimeSeries: (params) => 
    api.get('/metrics/prometheus/dropped-flows/timeseries', { params }),
  getAlertTypesStats: () => api.get('/metrics/prometheus/alert-types'),
}
```

### 3.3.6.2. WebSocket Integration

**WebSocket Helper:**
```javascript
export const createWebSocket = (endpoint) => {
  const WS_BASE_URL = import.meta.env.VITE_WS_URL || 'ws://localhost:5001/api/v1'
  const wsUrl = `${WS_BASE_URL}${endpoint}`
  return new WebSocket(wsUrl)
}
```

**Sử dụng trong Component:**
```javascript
useEffect(() => {
  const ws = createWebSocket('/stream/flows')
  
  ws.onopen = () => console.log('WebSocket connected')
  ws.onmessage = (event) => {
    const data = JSON.parse(event.data)
    // Update UI với data mới
    updateFlows(data)
  }
  ws.onerror = (error) => console.error('WebSocket error:', error)
  ws.onclose = () => console.warn('WebSocket closed')
  
  return () => ws.close()
}, [])
```

### 3.3.6.3. Error Handling

```javascript
try {
  const response = await flowsAPI.getAll(params)
  setFlows(response.data.items)
  setFlowsError(null)
} catch (err) {
  console.error('Failed to load flows:', err)
  setFlows([])
  setFlowsError(err.message || 'Failed to load flows')
}
```

## 3.3.7. Theme và Styling

### 3.3.7.1. Dark Theme Configuration

```javascript
const theme = createTheme({
  palette: {
    mode: 'dark',
    primary: {
      main: '#1976d2',
      light: '#42a5f5',
      dark: '#1565c0',
    },
    background: {
      default: '#121416',
      paper: '#1E2023',
    },
    text: {
      primary: '#E0E0E0',
      secondary: '#A8A8A8',
    },
  },
  components: {
    MuiPaper: {
      styleOverrides: {
        root: {
          backgroundColor: '#1E2023',
          backgroundImage: 'none',
        },
      },
    },
    MuiTableRow: {
      styleOverrides: {
        root: {
          '&:nth-of-type(even)': {
            backgroundColor: '#1B1D20',
          },
          '&:hover': {
            backgroundColor: '#26282C',
          },
        },
      },
    },
  },
})
```

### 3.3.7.2. Color Scheme

- **Primary**: Blue (#1976d2) - Cho các actions chính
- **Error**: Red (#d32f2f) - Cho alerts và errors
- **Warning**: Orange (#ed6c02) - Cho warnings
- **Success**: Green (#2e7d32) - Cho success states
- **Background**: Dark gray (#121416) - Nền chính
- **Paper**: Darker gray (#1E2023) - Nền cho cards, tables

## 3.3.8. Build và Deployment

### 3.3.8.1. Development

```bash
# Install dependencies
cd ui
npm install

# Start development server
npm run dev
```

Development server chạy tại `http://localhost:5000` với:
- Hot Module Replacement (HMR)
- Proxy cho API requests đến `http://localhost:5001`
- Proxy cho WebSocket connections

### 3.3.8.2. Production Build

```bash
# Build for production
npm run build
```

Build output nằm trong thư mục `dist/`, bao gồm:
- Optimized JavaScript bundles
- Minified CSS
- Static assets (images, fonts)

### 3.3.8.3. Docker Build

**Dockerfile (Multi-stage build):**
```dockerfile
# Build stage
FROM node:18-alpine AS builder
WORKDIR /app
COPY package*.json ./
RUN npm ci
COPY . .
RUN npm run build

# Production stage
FROM nginx:alpine
COPY --from=builder /app/dist /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf
EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]
```

**Nginx Configuration:**
```nginx
server {
    listen 80;
    server_name localhost;
    root /usr/share/nginx/html;
    index index.html;

    # Serve static files
    location / {
        try_files $uri $uri/ /index.html;
    }

    # Proxy API requests
    location /api {
        proxy_pass http://hubble-guard-api-server:5001;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection 'upgrade';
        proxy_set_header Host $host;
        proxy_cache_bypass $http_upgrade;
    }

    # Proxy WebSocket
    location /ws {
        proxy_pass http://hubble-guard-api-server:5001;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
    }
}
```

### 3.3.8.4. Environment Variables

Tạo file `.env` trong thư mục `ui/`:

```env
# API Server URL
VITE_API_URL=http://localhost:5001/api/v1
VITE_WS_URL=ws://localhost:5001/api/v1

# Grafana Configuration
VITE_GRAFANA_URL=http://localhost:3000
VITE_GRAFANA_DASHBOARD_UID=hubble-guard
VITE_GRAFANA_USE_PROXY=false
VITE_GRAFANA_USER=admin
VITE_GRAFANA_PASSWORD=admin
```

**Lưu ý**: Vite chỉ expose các biến môi trường bắt đầu bằng `VITE_` cho client-side code.

### 3.3.8.5. Kubernetes Deployment

UI được deploy như một Kubernetes Deployment với:
- **Image**: Container image chứa built static files
- **Service**: ClusterIP hoặc NodePort để expose UI
- **Ingress**: Optional, để expose UI ra ngoài cluster
- **ConfigMap**: Chứa nginx configuration nếu cần customize

**Helm Chart Values:**
```yaml
ui:
  enabled: true
  replicaCount: 2
  image:
    repository: your-registry/hubble-guard-ui
    tag: "1.0.0"
  service:
    type: ClusterIP
    port: 80
  ingress:
    enabled: true
    host: hubble-guard.example.com
```

## 3.3.9. Tối ưu hóa hiệu suất

### 3.3.9.1. Code Splitting

Vite tự động code splitting theo routes:
- Mỗi route được bundle riêng
- Chỉ load code cần thiết khi navigate

### 3.3.9.2. Lazy Loading

```javascript
import { lazy, Suspense } from 'react'

const Dashboard = lazy(() => import('./pages/Dashboard'))
const FlowViewer = lazy(() => import('./pages/FlowViewer'))

function App() {
  return (
    <Suspense fallback={<Loading />}>
      <Routes>
        <Route path="/" element={<Dashboard />} />
        <Route path="/flows" element={<FlowViewer />} />
      </Routes>
    </Suspense>
  )
}
```

### 3.3.9.3. Memoization

Sử dụng `React.memo` và `useMemo` để tránh re-render không cần thiết:

```javascript
const StatCard = React.memo(({ title, value, icon: Icon, color, loading }) => {
  // Component implementation
})

const chartData = useMemo(() => {
  return transformData(rawData)
}, [rawData])
```

### 3.3.9.4. Debouncing Search

```javascript
import { useDebouncedCallback } from 'use-debounce'

const debouncedSearch = useDebouncedCallback((value) => {
  setSearchTerm(value)
  setPage(0)
}, 300)

<TextField
  onChange={(e) => debouncedSearch(e.target.value)}
/>
```

## 3.3.10. Testing và Quality Assurance

### 3.3.10.1. Linting

```bash
npm run lint
```

ESLint được cấu hình để:
- Kiểm tra code style
- Phát hiện lỗi syntax
- Enforce React best practices

### 3.3.10.2. Type Checking

Mặc dù sử dụng JavaScript, có thể thêm TypeScript hoặc JSDoc comments để type checking:

```javascript
/**
 * @param {Object} params
 * @param {number} params.page
 * @param {number} params.limit
 * @returns {Promise<{data: {items: Flow[], total: number}}>}
 */
export const flowsAPI = {
  getAll: (params = {}) => api.get('/flows', { params }),
}
```

## 3.3.11. Tổng kết

Giao diện người dùng Hubble Guard được thiết kế và triển khai với các đặc điểm chính:

1. **Kiến trúc hiện đại**: Sử dụng React, Vite, Material-UI để tạo ứng dụng SPA hiệu quả
2. **Real-time updates**: Tích hợp WebSocket để cập nhật dữ liệu theo thời gian thực
3. **Responsive design**: Tương thích với nhiều thiết bị và kích thước màn hình
4. **Dark theme**: Giao diện tối giúp giảm mỏi mắt
5. **Performance**: Tối ưu hóa với code splitting, lazy loading, memoization
6. **Deployment**: Dễ dàng deploy với Docker và Kubernetes

UI hoạt động như một lớp trình bày độc lập, giao tiếp với API Server thông qua REST API và WebSocket, đảm bảo separation of concerns và khả năng scale riêng biệt.

