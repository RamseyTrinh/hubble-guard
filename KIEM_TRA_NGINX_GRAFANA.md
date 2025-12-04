# Hướng dẫn Kiểm tra Nginx Config cho Grafana

## 1. Kiểm tra nginx config trong pod

```bash
# Xem toàn bộ nginx config
kubectl exec -it deployment/hubble-guard-ui -n hubble-guard -- cat /etc/nginx/conf.d/default.conf

# Chỉ xem phần Grafana config
kubectl exec -it deployment/hubble-guard-ui -n hubble-guard -- grep -A 15 "location /grafana" /etc/nginx/conf.d/default.conf

# Kiểm tra resolver
kubectl exec -it deployment/hubble-guard-ui -n hubble-guard -- grep resolver /etc/nginx/conf.d/default.conf
```

**Phải thấy:**
- `resolver kube-dns.kube-system.svc.cluster.local coredns.kube-system.svc.cluster.local valid=10s;`
- `location /grafana/ {`
- `set $grafana "hubble-guard-grafana.hubble-guard.svc.cluster.local";`
- `proxy_pass http://$grafana:3000/;`

## 2. Test DNS resolution

```bash
# Test DNS cho Grafana service
kubectl exec -it deployment/hubble-guard-ui -n hubble-guard -- nslookup hubble-guard-grafana.hubble-guard.svc.cluster.local

# Phải thấy IP của Grafana service (ví dụ: 10.96.249.72)
```

## 3. Test kết nối trực tiếp đến Grafana (không qua nginx)

```bash
# Test từ UI pod đến Grafana service
kubectl exec -it deployment/hubble-guard-ui -n hubble-guard -- wget -O- http://hubble-guard-grafana.hubble-guard.svc.cluster.local:3000/api/health

# Hoặc dùng curl
kubectl exec -it deployment/hubble-guard-ui -n hubble-guard -- curl -v http://hubble-guard-grafana.hubble-guard.svc.cluster.local:3000/api/health
```

**Kết quả mong đợi:** HTTP 200 OK

## 4. Test nginx proxy (từ trong pod)

```bash
# Test nginx proxy đến Grafana
kubectl exec -it deployment/hubble-guard-ui -n hubble-guard -- wget -O- http://localhost/grafana/api/health

# Hoặc curl
kubectl exec -it deployment/hubble-guard-ui -n hubble-guard -- curl -v http://localhost/grafana/api/health
```

**Kết quả mong đợi:** HTTP 200 OK, response từ Grafana

## 5. Test từ bên ngoài (port-forward)

```bash
# Port forward UI service
kubectl port-forward -n hubble-guard service/hubble-guard-ui 8080:80

# Trong terminal khác, test Grafana proxy
curl -v http://localhost:8080/grafana/api/health

# Test Grafana dashboard endpoint
curl -v http://localhost:8080/grafana/d/hubble-guard
```

## 6. Kiểm tra nginx logs

```bash
# Xem nginx access logs
kubectl exec -it deployment/hubble-guard-ui -n hubble-guard -- tail -f /var/log/nginx/access.log

# Xem nginx error logs
kubectl exec -it deployment/hubble-guard-ui -n hubble-guard -- tail -f /var/log/nginx/error.log

# Hoặc xem logs của container
kubectl logs -f deployment/hubble-guard-ui -n hubble-guard
```

## 7. Kiểm tra nginx có chạy không

```bash
# Kiểm tra nginx process
kubectl exec -it deployment/hubble-guard-ui -n hubble-guard -- ps aux | grep nginx

# Test nginx config syntax
kubectl exec -it deployment/hubble-guard-ui -n hubble-guard -- nginx -t
```

## 8. Kiểm tra Grafana service

```bash
# Kiểm tra Grafana service tồn tại
kubectl get svc -n hubble-guard | grep grafana

# Kiểm tra Grafana pod đang chạy
kubectl get pods -n hubble-guard | grep grafana

# Kiểm tra Grafana logs
kubectl logs deployment/hubble-guard-grafana -n hubble-guard

# Test Grafana trực tiếp (port-forward)
kubectl port-forward -n hubble-guard service/hubble-guard-grafana 3000:3000
# Mở browser: http://localhost:3000
```

## 9. Debug nginx với curl chi tiết

```bash
# Exec vào pod
kubectl exec -it deployment/hubble-guard-ui -n hubble-guard -- sh

# Trong pod, test từng bước:
# 1. Test DNS
nslookup hubble-guard-grafana.hubble-guard.svc.cluster.local

# 2. Test kết nối trực tiếp
curl -v http://hubble-guard-grafana.hubble-guard.svc.cluster.local:3000/api/health

# 3. Test qua nginx
curl -v http://localhost/grafana/api/health

# 4. Xem nginx config
cat /etc/nginx/conf.d/default.conf | grep -A 20 grafana

# 5. Reload nginx (nếu cần)
nginx -s reload
```

## 10. Kiểm tra nginx có reload config mới không

```bash
# Xem nginx config timestamp
kubectl exec -it deployment/hubble-guard-ui -n hubble-guard -- ls -la /etc/nginx/conf.d/default.conf

# Nếu config cũ, cần restart pod
kubectl delete pod -n hubble-guard -l app.kubernetes.io/name=hubble-guard,app.kubernetes.io/component=ui
```

## Troubleshooting

### Nếu DNS không resolve được:

```bash
# Kiểm tra service name đúng chưa
kubectl get svc -n hubble-guard | grep grafana

# Kiểm tra namespace
kubectl get svc -A | grep hubble-guard-grafana
```

### Nếu nginx proxy trả về 502 Bad Gateway:

- DNS không resolve được → Kiểm tra resolver và FQDN
- Grafana service không chạy → Kiểm tra Grafana pod
- Port sai → Kiểm tra Grafana service port (phải là 3000)

### Nếu nginx proxy trả về 404:

- Path rewrite sai → Kiểm tra `proxy_pass` có trailing slash `/` chưa
- Location block không match → Kiểm tra `location /grafana/` có đúng không

### Nếu vẫn thấy localhost:3000 trong UI:

- Image chưa rebuild → Cần rebuild và push image mới
- Browser cache → Clear cache hoặc hard refresh (Ctrl+Shift+R)

