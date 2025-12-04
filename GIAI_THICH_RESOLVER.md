# Giải thích về Resolver trong Nginx

## Resolver là gì?

`resolver` trong nginx là DNS server mà nginx sẽ dùng để resolve (phân giải) tên miền thành IP address.

```nginx
resolver 10.96.0.10 valid=10s;
```

- `10.96.0.10`: IP của DNS server (trong K8s là kube-dns/coredns)
- `valid=10s`: Cache kết quả DNS trong 10 giây

## Tại sao cần Resolver?

Khi bạn dùng **variable** trong `proxy_pass`, nginx **KHÔNG THỂ** resolve DNS lúc startup. Nó chỉ resolve khi có request đến.

```nginx
# ❌ KHÔNG hoạt động - nginx không thể resolve lúc startup
location /api {
    proxy_pass http://hubble-guard-api-server.hubble-guard.svc.cluster.local:5001;
}

# ✅ Hoạt động - dùng variable + resolver
location /api {
    set $backend "hubble-guard-api-server.hubble-guard.svc.cluster.local";
    proxy_pass http://$backend:5001;
}
```

**Lý do**: Khi dùng variable, nginx cần resolver để resolve DNS động mỗi khi có request.

## Vấn đề với Hardcode IP

IP `10.96.0.10` là IP của kube-dns service trong cụm K8s hiện tại. **Sang cụm K8s khác, IP này có thể khác!**

### Cách tìm IP của kube-dns trong cụm K8s:

```bash
# Cách 1: Tìm service kube-dns
kubectl get svc -n kube-system | grep kube-dns

# Cách 2: Lấy IP trực tiếp
kubectl get svc kube-dns -n kube-system -o jsonpath='{.spec.clusterIP}'

# Cách 3: Nếu dùng CoreDNS (phổ biến hơn)
kubectl get svc -n kube-system | grep coredns
kubectl get svc coredns -n kube-system -o jsonpath='{.spec.clusterIP}'
```

## Giải pháp: Làm Resolver Dynamic

Có 3 cách để không phải hardcode IP:

### Cách 1: Dùng Service Name thay vì IP (Khuyến nghị)

Thay vì dùng IP, dùng service name của kube-dns/coredns:

```nginx
# Dùng service name - sẽ resolve được trong K8s
resolver kube-dns.kube-system.svc.cluster.local valid=10s;
# Hoặc
resolver coredns.kube-system.svc.cluster.local valid=10s;
```

**Vấn đề**: Nginx resolver **KHÔNG THỂ** resolve service name lúc startup nếu không có resolver khác. Đây là vấn đề "chicken and egg".

### Cách 2: Dùng envsubst để inject IP động (Tốt nhất)

Sửa Dockerfile để inject IP từ environment variable:

**1. Tạo nginx.conf.template:**

```nginx
server {
    listen 80;
    server_name localhost;
    root /usr/share/nginx/html;
    index index.html;

    # Resolver sẽ được thay thế bởi envsubst
    resolver ${KUBE_DNS_IP} valid=10s;

    location / {
        try_files $uri $uri/ /index.html;
    }

    location /api {
        set $backend "hubble-guard-api-server.hubble-guard.svc.cluster.local";
        proxy_pass http://$backend:5001;
        # ... rest of config
    }
}
```

**2. Sửa Dockerfile:**

```dockerfile
FROM nginx:alpine

# Install envsubst
RUN apk add --no-cache gettext

# Copy template
COPY nginx.conf.template /etc/nginx/templates/default.conf.template

# Copy built files
COPY --from=builder /app/dist /usr/share/nginx/html

# Entrypoint script
COPY docker-entrypoint.sh /docker-entrypoint.sh
RUN chmod +x /docker-entrypoint.sh

ENTRYPOINT ["/docker-entrypoint.sh"]
CMD ["nginx", "-g", "daemon off;"]
```

**3. Tạo docker-entrypoint.sh:**

```bash
#!/bin/sh
set -e

# Tìm IP của kube-dns/coredns
KUBE_DNS_IP=${KUBE_DNS_IP:-$(getent hosts kube-dns.kube-system.svc.cluster.local | awk '{print $1}' | head -1)}

# Nếu không tìm được, thử coredns
if [ -z "$KUBE_DNS_IP" ]; then
    KUBE_DNS_IP=$(getent hosts coredns.kube-system.svc.cluster.local | awk '{print $1}' | head -1)
fi

# Nếu vẫn không có, dùng default
if [ -z "$KUBE_DNS_IP" ]; then
    KUBE_DNS_IP="10.96.0.10"
fi

export KUBE_DNS_IP

# Thay thế variable trong template
envsubst '${KUBE_DNS_IP}' < /etc/nginx/templates/default.conf.template > /etc/nginx/conf.d/default.conf

# Start nginx
exec "$@"
```

**4. Cập nhật Helm deployment để set env:**

```yaml
env:
  - name: KUBE_DNS_IP
    valueFrom:
      fieldRef:
        fieldPath: status.hostIP  # Hoặc dùng init container để tìm
```

### Cách 3: Dùng Init Container (Phức tạp hơn)

Tạo init container để tìm IP và mount vào main container.

## Giải pháp Đơn giản nhất (Khuyến nghị cho production)

**Dùng service name với fallback:**

```nginx
# Thử resolve service name, nếu không được thì dùng IP
resolver kube-dns.kube-system.svc.cluster.local coredns.kube-system.svc.cluster.local 10.96.0.10 valid=10s;
```

Nginx sẽ thử từ trái sang phải, cái nào resolve được thì dùng.

## So sánh các cách

| Cách | Ưu điểm | Nhược điểm |
|------|---------|------------|
| Hardcode IP | Đơn giản | Không portable, phải sửa khi sang cluster khác |
| Service name | Portable | Có thể không hoạt động nếu resolver chưa sẵn sàng |
| envsubst | Portable, linh hoạt | Phức tạp hơn, cần thêm script |
| Init container | Rất linh hoạt | Phức tạp nhất |

## Khuyến nghị

**Cho development/testing**: Dùng hardcode IP, dễ debug

**Cho production**: Dùng envsubst với entrypoint script để tự động tìm IP

