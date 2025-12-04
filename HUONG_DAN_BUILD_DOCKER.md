# Hướng dẫn Build và Push Docker Images

## Cấu hình

Trong file `Makefile`, các biến sau đã được cấu hình:
- `DOCKER_USERNAME=ramseytrinh338` - Tên người dùng Docker Hub
- `VERSION=1.0.0` - Phiên bản image

Bạn có thể thay đổi các giá trị này trong Makefile hoặc override khi chạy lệnh.

## Đăng nhập Docker Hub

Trước khi push images, bạn cần đăng nhập vào Docker Hub:

```bash
echo "YOUR_DOCKER_PASSWORD" | docker login -u "YOUR_USERNAME" --password-stdin
```

## Build từng image riêng lẻ

### Build hubble-guard
```bash
make docker-build-guard
```

### Build hubble-guard-api
```bash
make docker-build-api
```

### Build hubble-guard-ui
```bash
make docker-build-ui
```

## Build tất cả images cùng lúc

```bash
make docker-build-all
```

## Push từng image riêng lẻ

### Push hubble-guard
```bash
make docker-push-guard
```

### Push hubble-guard-api
```bash
make docker-push-api
```

### Push hubble-guard-ui
```bash
make docker-push-ui
```

## Build và Push tất cả images cùng lúc

```bash
make docker-push-all
```

Lệnh này sẽ:
1. Build cả 3 images (hubble-guard, hubble-guard-api, hubble-guard-ui)
2. Push tất cả lên Docker Hub với tag `VERSION` và `latest`

## Override biến

Bạn có thể override các biến khi chạy lệnh:

```bash
# Thay đổi version
make docker-build-all VERSION=1.0.1

# Thay đổi Docker username
make docker-push-all DOCKER_USERNAME=your-username
```

## Kiểm tra images đã build

```bash
docker images | grep hubble-guard
```

## Xem tất cả các lệnh có sẵn

```bash
make help
```

