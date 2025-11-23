# Mã Giả (Pseudocode) cho Các Rule Phát Hiện Anomaly

Tài liệu này mô tả mã giả ngắn gọn cho các rule phát hiện anomaly đã được cấu hình.

## 1. Rule DDoS (Real-time từ Hubble Flows)
**Threshold**: `multiplier = 3.0` | **Baseline**: 1 phút | **Check**: 10 giây

```
FUNCTION Evaluate(flow):
    namespace = ExtractNamespace(flow)
    
    // Thu thập baseline trong 1 phút đầu
    IF baseline[namespace] NOT EXISTS:
        flowCounts[namespace]++
        IF elapsed >= 1 phút:
            baseline[namespace] = flowCounts[namespace] / 1 phút
        RETURN NULL
    
    // So sánh với baseline
    flowCounts[namespace]++
    IF elapsed >= 10 giây:
        currentRate = flowCounts[namespace] / 10 giây
        multiplier = currentRate / baseline[namespace]
        
        IF multiplier > 3.0:
            EMIT Alert("DDoS attack: multiplier x baseline")
        
        flowCounts[namespace] = 0
```

---

## 2. Rule Traffic Spike (Từ Prometheus)
**Threshold**: `multiplier = 3.0` | **Check**: 10 giây | **Baseline**: 1 phút

```
FUNCTION CheckNamespace(namespace):
    currentRate = QueryPrometheus("rate(hubble_flows_total{namespace='ns'}[1m])")
    
    // Thu thập baseline
    IF baseline[namespace] NOT EXISTS:
        baselineRates[namespace].Append(currentRate)
        IF elapsed >= 1 phút:
            baseline[namespace] = Average(baselineRates[namespace])
        RETURN
    
    // Phát hiện traffic spike
    multiplier = currentRate / baseline[namespace]
    IF multiplier > 3.0:
        EMIT Alert("Traffic spike: multiplier x baseline (possible DDoS)")
```

---

## 3. Rule Traffic Death (Từ Prometheus)
**Check**: 10 giây | **Baseline**: 1 phút

```
FUNCTION CheckNamespace(namespace):
    currentRate = QueryPrometheus("rate(hubble_flows_total{namespace='ns'}[1m])")
    
    // Thu thập baseline
    IF baseline[namespace] NOT EXISTS:
        baselineRates[namespace].Append(currentRate)
        IF elapsed >= 1 phút:
            baseline[namespace] = Average(baselineRates[namespace])
        RETURN
    
    // Phát hiện traffic = 0
    IF currentRate == 0 AND baseline[namespace] > 0:
        EMIT Alert("Traffic death: Service may be down!")
```

---

## 4. Rule Block Connection (Từ Prometheus)
**Threshold**: `10 DROP flows/phút` | **Check**: 10 giây

```
FUNCTION CheckNamespace(namespace):
    dropCount = QueryPrometheus("sum(increase(hubble_flows_by_verdict_total{verdict='DROP'}[1m]))")
    
    IF dropCount > 10:
        EMIT Alert("Blocked connections: dropCount DROP flows")
```

---

## 5. Rule Port Scan (Từ Prometheus)
**Threshold**: `50 distinct ports/10s` | **Check**: 10 giây

```
FUNCTION CheckNamespace(namespace):
    FOR EACH sample IN QueryPrometheus("portscan_distinct_ports_10s{namespace='ns'}"):
        distinctPorts = sample.Value
        
        IF distinctPorts > 50:
            EMIT Alert("Port scanning: distinctPorts ports from sourceIP to destIP")
```

---

## 6. Rule Namespace Access (Từ Prometheus)
**Forbidden**: `["kube-system", "monitoring", "security"]` | **Check**: 10 giây

```
FUNCTION CheckForbiddenNamespace(forbiddenNS):
    FOR EACH sample IN QueryPrometheus("hubble_namespace_access_total{dest_namespace='forbiddenNS'}[1m]"):
        sourceNS = sample.source_namespace
        destNS = sample.dest_namespace
        destService = sample.dest_service
        
        IF sourceNS != destNS AND forbiddenNS.Contains(destNS):
            isDNS = (destService == "kube-dns" OR destService == "coredns")
            
            IF isDNS AND sourceNS != "kube-system":
                EMIT Alert("Unauthorized DNS access from sourceNS")
            ELSE:
                EMIT Alert("Unauthorized access to destNS from sourceNS")
```

---

## 7. Rule Suspicious Outbound (Từ Prometheus)
**Threshold**: `10 connections/phút` | **Ports**: `[23, 135, 445, 1433, 3306, 5432]` | **Check**: 10 giây

```
FUNCTION CheckNamespace(namespace):
    FOR EACH port IN [23, 135, 445, 1433, 3306, 5432]:
        count = QueryPrometheus("sum(increase(hubble_suspicious_outbound_total{port='port'}[1m]))")
        
        IF count > 10:
            EMIT Alert("Suspicious outbound: count connections to port port")
```

---

## 8. Rule TCP Drop Surge (Real-time từ Hubble Flows)
**Threshold**: `10 drops/phút`

```
FUNCTION Evaluate(flow):
    IF flow.Verdict != DROPPED:
        RETURN NULL
    
    namespace = ExtractNamespace(flow)
    dropCounts[namespace]++
    
    IF elapsed >= 1 phút:
        IF dropCounts[namespace] > 10:
            EMIT Alert("TCP drop surge: dropCounts drops")
        dropCounts[namespace] = 0
```

---

## Tổng Kết

**Real-time Rules** (xử lý từng flow): DDoS, TCP Drop Surge  
**Periodic Rules** (truy vấn Prometheus mỗi 10s): Traffic Spike, Traffic Death, Block Connection, Port Scan, Namespace Access, Suspicious Outbound

