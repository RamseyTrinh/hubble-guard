# Hướng dẫn Fix UI không kết nối được API trong K8s

## Vấn đề

1. UI đang gọi API đến `localhost:5001` thay vì service name trong K8s, dẫn đến lỗi "Network Error".
2. UI đang gọi Grafana đến `localhost:3000` thay vì service name trong K8s, dẫn đến lỗi "localhost refused to connect".

## Nguyên nhân

1. **Vite env vars chỉ hoạt động lúc build time**: Environment variables `VITE_API_URL` và `VITE_WS_URL` chỉ được inject vào code lúc build, không phải runtime.
2. **nginx.conf proxy sai service**: Đang proxy đến `hubble-guard-anomaly-detector:8080` thay vì `hubble-guard-api-server:5001`.
3. **UI code hardcode localhost**: Một số file vẫn hardcode `localhost:5001`.

## Giải pháp đã áp dụng

### 1. Sửa nginx.conf
- Đổi proxy từ `hubble-guard-anomaly-detector:8080` → `hubble-guard-api-server:5001`
- **Quan trọng**: Dùng FQDN đầy đủ `hubble-guard-api-server.hubble-guard.svc.cluster.local` thay vì service name ngắn
- Thêm DNS resolver: `resolver kube-dns.kube-system.svc.cluster.local coredns.kube-system.svc.cluster.local valid=10s;`
- Dùng variable `$backend` với resolver để nginx có thể resolve DNS động
- **Thêm Grafana proxy**: Location `/grafana` proxy đến `hubble-guard-grafana.hubble-guard.svc.cluster.local:3000`
- Thêm CORS headers cho API
- Thêm WebSocket support với timeouts phù hợp
- Remove X-Frame-Options header cho Grafana để cho phép embed

### 2. Sửa UI code
- `api.js`: Dùng relative path `/api/v1` trong production (nginx sẽ proxy), absolute URL trong development
- `FlowViewer.jsx`: Dùng `WS_BASE_URL` từ config thay vì hardcode `localhost:5001`
- `GrafanaEmbed.jsx`: Dùng relative path `/grafana` trong production (nginx sẽ proxy), absolute URL trong development
- Export `WS_BASE_URL` để các component khác có thể sử dụng

## Các bước deploy lại

### Bước 1: Rebuild UI image

```bash
# Từ thư mục gốc
make docker-build-ui

```

### Bước 2: Push image lên Docker Hub

```bash
make docker-push-ui

```

### Bước 3: Restart UI deployment

```bash
# Xóa pod để nó tự tạo lại với image mới (nhớ thêm namespace)
kubectl delete pod -n hubble-guard -l app.kubernetes.io/name=hubble-guard,app.kubernetes.io/component=ui

# Hoặc rollout restart
kubectl rollout restart deployment/hubble-guard-ui -n hubble-guard

# Kiểm tra rollout status
kubectl rollout status deployment/hubble-guard-ui -n hubble-guard
```

### Bước 4: Kiểm tra logs

```bash
# Xem logs của UI pod (nhớ thêm namespace)
kubectl logs -f deployment/hubble-guard-ui -n hubble-guard

# Xem logs của API server
kubectl logs -f deployment/hubble-guard-api-server -n hubble-guard

# Kiểm tra nginx error logs (nếu có)
kubectl exec -it deployment/hubble-guard-ui -n hubble-guard -- cat /var/log/nginx/error.log
```

### Bước 5: Test kết nối

```bash
# Port forward UI
kubectl port-forward -n hubble-guard service/hubble-guard-ui 8080:80

# Trong terminal khác, test API qua nginx proxy
curl http://localhost:8080/api/v1/health

# Test Grafana qua nginx proxy
curl http://localhost:8080/grafana/api/health

# Test trực tiếp API server (nếu cần)
kubectl port-forward -n hubble-guard service/hubble-guard-api-server 5001:5001
curl http://localhost:5001/api/v1/health

# Test trực tiếp Grafana (nếu cần)
kubectl port-forward -n hubble-guard service/hubble-guard-grafana 3000:3000
curl http://localhost:3000/api/health
```

## Kiểm tra cấu hình

### Kiểm tra service name đúng chưa

```bash
# Kiểm tra service trong namespace
kubectl get svc -n hubble-guard | grep api-server
# Phải thấy: hubble-guard-api-server

# Kiểm tra service details
kubectl get svc hubble-guard-api-server -n hubble-guard -o yaml

# Kiểm tra endpoint
kubectl get endpoints hubble-guard-api-server -n hubble-guard
```

### Kiểm tra nginx config trong pod

```bash
# Exec vào UI pod (nhớ thêm namespace nếu cần)
kubectl exec -it deployment/hubble-guard-ui -n hubble-guard -- cat /etc/nginx/conf.d/default.conf

# Phải thấy:
# - resolver 10.96.0.10 valid=10s;
# - set $backend "hubble-guard-api-server.hubble-guard.svc.cluster.local";
# - proxy_pass http://$backend:5001;
```

### Test từ trong pod

```bash
# Test DNS resolution cho API
kubectl exec -it deployment/hubble-guard-ui -n hubble-guard -- nslookup hubble-guard-api-server.hubble-guard.svc.cluster.local

# Test DNS resolution cho Grafana
kubectl exec -it deployment/hubble-guard-ui -n hubble-guard -- nslookup hubble-guard-grafana.hubble-guard.svc.cluster.local

# Test kết nối đến API bằng FQDN
kubectl exec -it deployment/hubble-guard-ui -n hubble-guard -- wget -O- http://hubble-guard-api-server.hubble-guard.svc.cluster.local:5001/api/v1/health

# Test kết nối đến Grafana bằng FQDN
kubectl exec -it deployment/hubble-guard-ui -n hubble-guard -- wget -O- http://hubble-guard-grafana.hubble-guard.svc.cluster.local:3000/api/health

# Test qua nginx proxy (từ trong pod)
kubectl exec -it deployment/hubble-guard-ui -n hubble-guard -- wget -O- http://localhost/api/v1/health
kubectl exec -it deployment/hubble-guard-ui -n hubble-guard -- wget -O- http://localhost/grafana/api/health
```

## Troubleshooting

### Nếu vẫn lỗi "Network Error"

1. **Kiểm tra service name và namespace**:
   ```bash
   kubectl get svc -A | grep hubble-guard-api-server
   # Phải thấy service trong namespace hubble-guard
   ```

2. **Kiểm tra namespace**: Đảm bảo UI và API ở cùng namespace
   ```bash
   kubectl get pods -n hubble-guard | grep hubble-guard
   # Cả UI và API server phải ở cùng namespace
   ```

3. **Kiểm tra nginx config có dùng FQDN**:
   ```bash
   kubectl exec -it deployment/hubble-guard-ui -n hubble-guard -- grep -A 3 "location /api" /etc/nginx/conf.d/default.conf
   # Phải thấy: hubble-guard-api-server.hubble-guard.svc.cluster.local
   ```

3. **Kiểm tra DNS resolution trong pod**:
   ```bash
   # Test với service name ngắn (có thể không resolve được)
   kubectl exec -it deployment/hubble-guard-ui -n hubble-guard -- nslookup hubble-guard-api-server
   
   # Test với FQDN (phải resolve được)
   kubectl exec -it deployment/hubble-guard-ui -n hubble-guard -- nslookup hubble-guard-api-server.hubble-guard.svc.cluster.local
   
   # Kiểm tra resolver trong nginx config
   kubectl exec -it deployment/hubble-guard-ui -n hubble-guard -- grep resolver /etc/nginx/conf.d/default.conf
   ```

4. **Kiểm tra API server có chạy không**:
   ```bash
   kubectl get pods -n hubble-guard | grep api-server
   kubectl logs deployment/hubble-guard-api-server -n hubble-guard
   
   # Test API server từ pod khác
   kubectl run -it --rm debug --image=curlimages/curl --restart=Never -n hubble-guard -- \
     curl http://hubble-guard-api-server.hubble-guard.svc.cluster.local:5001/api/v1/health
   ```

### Nếu WebSocket không hoạt động

1. Kiểm tra nginx config có WebSocket support:
   ```bash
   kubectl exec deployment/hubble-guard-ui -n hubble-guard -- grep -A 5 "upgrade" /etc/nginx/conf.d/default.conf
   # Phải thấy: proxy_set_header Upgrade và Connection "upgrade"
   ```

2. Kiểm tra API server có hỗ trợ WebSocket:
   ```bash
   kubectl logs deployment/hubble-guard-api-server -n hubble-guard | grep -i websocket
   ```

3. Test WebSocket connection:
   ```bash
   # Port forward UI
   kubectl port-forward -n hubble-guard service/hubble-guard-ui 8080:80
   
   # Test WebSocket (cần tool như wscat hoặc websocat)
   # wscat -c ws://localhost:8080/api/v1/stream/flows
   ```

### Nếu Grafana không load được

1. **Kiểm tra Grafana service**:
   ```bash
   kubectl get svc -n hubble-guard | grep grafana
   kubectl get pods -n hubble-guard | grep grafana
   ```

2. **Kiểm tra nginx config có Grafana proxy**:
   ```bash
   # Xem toàn bộ Grafana config
   kubectl exec -it deployment/hubble-guard-ui -n hubble-guard -- grep -A 15 "location /grafana" /etc/nginx/conf.d/default.conf
   
   # Phải thấy:
   # - location /grafana/ {
   # - set $grafana "hubble-guard-grafana.hubble-guard.svc.cluster.local";
   # - proxy_pass http://$grafana:3000/;
   ```

3. **Test DNS resolution**:
   ```bash
   kubectl exec -it deployment/hubble-guard-ui -n hubble-guard -- nslookup hubble-guard-grafana.hubble-guard.svc.cluster.local
   # Phải resolve được IP (ví dụ: 10.96.249.72)
   ```

4. **Test kết nối trực tiếp đến Grafana (không qua nginx)**:
   ```bash
   kubectl exec -it deployment/hubble-guard-ui -n hubble-guard -- curl -v http://hubble-guard-grafana.hubble-guard.svc.cluster.local:3000/api/health
   # Phải trả về HTTP 200 OK
   ```

5. **Test nginx proxy**:
   ```bash
   # Test từ trong pod
   kubectl exec -it deployment/hubble-guard-ui -n hubble-guard -- curl -v http://localhost/grafana/api/health
   # Phải trả về HTTP 200 OK từ Grafana
   
   # Test từ bên ngoài (port-forward)
   kubectl port-forward -n hubble-guard service/hubble-guard-ui 8080:80
   curl -v http://localhost:8080/grafana/api/health
   ```

6. **Kiểm tra nginx logs**:
   ```bash
   # Error logs
   kubectl exec -it deployment/hubble-guard-ui -n hubble-guard -- tail -20 /var/log/nginx/error.log
   
   # Access logs
   kubectl exec -it deployment/hubble-guard-ui -n hubble-guard -- tail -20 /var/log/nginx/access.log
   ```

7. **Kiểm tra nginx config syntax**:
   ```bash
   kubectl exec -it deployment/hubble-guard-ui -n hubble-guard -- nginx -t
   # Phải thấy: "syntax is ok" và "test is successful"
   ```

8. **Kiểm tra Grafana logs**:
   ```bash
   kubectl logs deployment/hubble-guard-grafana -n hubble-guard --tail=50
   ```

9. **Kiểm tra X-Frame-Options header**:
   ```bash
   kubectl exec -it deployment/hubble-guard-ui -n hubble-guard -- curl -I http://localhost/grafana/api/health
   # Phải không thấy X-Frame-Options header (đã bị proxy_hide_header remove)
   ```

10. **Kiểm tra image đã rebuild chưa**:
    ```bash
    # Xem image tag hiện tại
    kubectl get deployment hubble-guard-ui -n hubble-guard -o jsonpath='{.spec.template.spec.containers[0].image}'
    
    # Nếu cần, force pull image mới
    kubectl rollout restart deployment/hubble-guard-ui -n hubble-guard
    kubectl rollout status deployment/hubble-guard-ui -n hubble-guard
    ```

**Xem file `KIEM_TRA_NGINX_GRAFANA.md` để có hướng dẫn chi tiết hơn.**

## Lưu ý quan trọng

### Về DNS resolution trong Kubernetes

- **Service name ngắn** (`hubble-guard-api-server`) chỉ hoạt động trong cùng namespace và không hoạt động với nginx resolver
- **FQDN đầy đủ** (`hubble-guard-api-server.hubble-guard.svc.cluster.local`) là bắt buộc khi dùng nginx resolver
- **Resolver**: Dùng service names (`kube-dns.kube-system.svc.cluster.local` hoặc `coredns.kube-system.svc.cluster.local`) thay vì hardcode IP để portable hơn
- **Variable trong proxy_pass**: Phải dùng `$backend` variable với resolver, không thể dùng trực tiếp hostname

### Về Grafana embedding

- **X-Frame-Options**: Grafana mặc định block embedding, nginx proxy sẽ remove header này bằng `proxy_hide_header X-Frame-Options`
- **Path rewrite**: Nginx sẽ rewrite `/grafana/xxx` thành `/xxx` khi proxy đến Grafana
- **Production vs Development**: Trong production, UI dùng relative path `/grafana`, trong development có thể dùng absolute URL hoặc proxy

### Cách tìm resolver IP (nếu cần fallback)

```bash
# Tìm IP của kube-dns
kubectl get svc -n kube-system | grep kube-dns
# Hoặc
kubectl get svc kube-dns -n kube-system -o jsonpath='{.spec.clusterIP}'

# Tìm IP của coredns (nếu dùng CoreDNS)
kubectl get svc -n kube-system | grep coredns
kubectl get svc coredns -n kube-system -o jsonpath='{.spec.clusterIP}'
```

## Alternative: Dùng Ingress với path rewrite

Nếu muốn truy cập từ bên ngoài, có thể setup Ingress:

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: hubble-guard-ui
spec:
  ingressClassName: nginx
  rules:
    - host: hubble-guard.local
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: hubble-guard-ui
                port:
                  number: 80
          - path: /api
            pathType: Prefix
            backend:
              service:
                name: hubble-guard-api-server
                port:
                  number: 5001
```

