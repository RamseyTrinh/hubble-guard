# Quick Start Guide

Hướng dẫn nhanh để chạy UI trên môi trường development.

## Bước 1: Cài đặt dependencies

```bash
cd ui
npm install
```

## Bước 2: Cấu hình environment variables

Tạo file `.env` trong thư mục `ui/`:

```env
VITE_API_URL=http://localhost:5001/api/v1
VITE_WS_URL=ws://localhost:5001/api/v1
```

**Lưu ý**: Backend cần expose các API endpoints như đã mô tả trong `README.md`. Nếu backend chưa có API, UI sẽ hiển thị lỗi khi gọi API.

## Bước 3: Chạy development server

```bash
npm run dev
```

UI sẽ chạy tại `http://localhost:5000`

## Bước 4: Build cho production

```bash
npm run build
```

Output sẽ ở trong thư mục `dist/`

## Bước 5: Build Docker image

```bash
docker build -t hubble-ui:1.0.0 .
```

## Bước 6: Test Docker image

```bash
docker run -p 5000:80 hubble-ui:1.0.0
```

Truy cập `http://localhost:5000`

## Troubleshooting

### Lỗi kết nối API

Nếu thấy lỗi "Network Error" hoặc "Failed to fetch", kiểm tra:

1. Backend có đang chạy không?
2. Backend có expose API endpoints không?
3. CORS có được config đúng không?
4. `VITE_API_URL` trong `.env` có đúng không?

### Lỗi WebSocket

Nếu WebSocket không kết nối được:

1. Kiểm tra backend có hỗ trợ WebSocket không
2. Kiểm tra `VITE_WS_URL` trong `.env`
3. Kiểm tra firewall/proxy có block WebSocket không

### Lỗi build

Nếu gặp lỗi khi build:

```bash
# Xóa node_modules và reinstall
rm -rf node_modules package-lock.json
npm install

# Hoặc dùng yarn
yarn install
```

