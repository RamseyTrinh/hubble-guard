# Hướng dẫn Truy cập Hubble Guard UI

## Cách 1: Port Forward (Nhanh nhất - Khuyến nghị cho test)

Chạy lệnh sau để forward port từ pod ra máy local:

```bash
kubectl port-forward service/hubble-guard-ui 8080:80
```

Sau đó mở trình duyệt và truy cập:
```
http://localhost:8080
```

**Lưu ý:** Lệnh này sẽ chạy ở foreground. Để chạy ở background, thêm `&`:
```bash
kubectl port-forward service/hubble-guard-ui 8080:80 &
```

Hoặc forward trực tiếp từ pod:
```bash
kubectl port-forward pod/hubble-guard-ui-7d75d9844b-snpqd 8080:80
```

---

## Cách 2: Kích hoạt Ingress (Khuyến nghị cho production)

### Bước 1: Kiểm tra Ingress Controller đã cài đặt chưa

```bash
kubectl get ingressclass
```

Nếu chưa có, cài đặt Nginx Ingress Controller:
```bash
kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/controller-v1.8.1/deploy/static/provider/cloud/deploy.yaml
```

### Bước 2: Cập nhật values.yaml để bật Ingress

Sửa file `helm/hubble-guard/values.yaml`:

```yaml
ui:
  ingress:
    enabled: true
    className: "nginx"
    hosts:
      - host: hubble-guard-ui.local  # Hoặc domain của bạn
        paths:
          - path: /
            pathType: Prefix
```

### Bước 3: Upgrade Helm chart

```bash
helm upgrade hubble-guard ./helm/hubble-guard --namespace hubble-guard
```

### Bước 4: Thêm host vào /etc/hosts (nếu dùng domain local)

```bash
echo "<NODE_IP> hubble-guard-ui.local" | sudo tee -a /etc/hosts
```

Sau đó truy cập: `http://hubble-guard-ui.local`

---

## Cách 3: Đổi Service Type thành NodePort

### Tạo file patch:

```yaml
# ui-service-nodeport.yaml
apiVersion: v1
kind: Service
metadata:
  name: hubble-guard-ui
spec:
  type: NodePort
  ports:
    - port: 80
      targetPort: 80
      nodePort: 30080  # Port trên node (30000-32767)
```

### Apply patch:

```bash
kubectl patch service hubble-guard-ui -p '{"spec":{"type":"NodePort","ports":[{"port":80,"targetPort":80,"nodePort":30080}]}}'
```

Hoặc edit trực tiếp:
```bash
kubectl edit service hubble-guard-ui
# Đổi type: ClusterIP thành type: NodePort
```

Sau đó truy cập: `http://<NODE_IP>:30080`

---

## Cách 4: Đổi Service Type thành LoadBalancer (Cloud)

Nếu cluster chạy trên cloud (AWS, GCP, Azure), có thể dùng LoadBalancer:

```bash
kubectl patch service hubble-guard-ui -p '{"spec":{"type":"LoadBalancer"}}'
```

Sau đó kiểm tra EXTERNAL-IP:
```bash
kubectl get service hubble-guard-ui
```

Truy cập qua EXTERNAL-IP: `http://<EXTERNAL-IP>`

---

## Kiểm tra namespace

Nếu các lệnh trên không hoạt động, có thể service ở namespace khác. Kiểm tra:

```bash
kubectl get svc -A | grep hubble-guard-ui
```

Sau đó thêm `-n <namespace>` vào các lệnh, ví dụ:
```bash
kubectl port-forward -n hubble-guard service/hubble-guard-ui 8080:80
```

---

## Truy cập API Server (nếu cần)

Tương tự, có thể port-forward API server:

```bash
kubectl port-forward service/hubble-guard-api-server 5001:5001
```

Truy cập: `http://localhost:5001/api/v1/health`

