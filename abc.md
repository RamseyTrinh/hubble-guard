```bash
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
- role: worker
- role: worker
networking:
  disableDefaultCNI: true
  kubeProxyMode: none
```

```bash
rules:
  - name: <tên của rule>
    enabled: <có kích hoạt luật hay không>
    severity: <mức độ của luật>
    description: <mô tả chi tiết về chức năng của luật>
    thresholds:
      multiplier: <ngưỡng so sánh so với baseline>

```

```log
Nov 25 14:08:48.230: default/demo-frontend-85875bc649-6955f:59962 (ID:2300) -> default/demo-api-69bf544bbf-8v7wm:8080 (ID:36333) to-endpoint FORWARDED (TCP Flags: ACK)
Nov 25 14:08:48.230: default/demo-frontend-85875bc649-6955f:59962 (ID:2300) -> default/demo-api-69bf544bbf-8v7wm:8080 (ID:36333) to-endpoint FORWARDED (TCP Flags: ACK, PSH)
Nov 25 14:08:48.230: default/demo-frontend-85875bc649-6955f:59962 (ID:2300) <> default/demo-api-69bf544bbf-8v7wm (ID:36333) pre-xlate-rev TRACED (TCP)
Nov 25 14:08:48.831: default/demo-frontend-85875bc649-6955f:59962 (ID:2300) <- default/demo-api-69bf544bbf-8v7wm:8080 (ID:36333) to-overlay FORWARDED (TCP Flags: ACK, PSH)
Nov 25 14:08:48.831: default/demo-frontend-85875bc649-6955f:59962 (ID:2300) <- default/demo-api-69bf544bbf-8v7wm:8080 (ID:36333) to-overlay FORWARDED (TCP Flags: ACK, FIN)
Nov 25 14:08:48.831: default/demo-frontend-85875bc649-6955f:59962 (ID:2300) -> default/demo-api-69bf544bbf-8v7wm:8080 (ID:36333) to-endpoint FORWARDED (TCP Flags: ACK, FIN)
```

```js
document.getElementById('ddos').onclick = () => {
          clearAllTimers();
          activeScenario = 'ddos';
          log('Starting DDoS Flood scenario (very high frequency)', 'error');
          const timer = setInterval(() => {
            if (activeScenario !== 'ddos') {
              clearInterval(timer);
              return;
            }
            send(`status/${pick([200, 204, 404])}`);
          }, 30); // 30ms = ~33 requests/second
          timers.push(timer);
        };
```

```js
document.getElementById('portscan').onclick = () => {
          clearAllTimers();
          activeScenario = 'portscan';
          log('Starting Port Scan scenario (scanning multiple endpoints rapidly)', 'warning');
          const endpoints = ['status/200', 'status/204', 'status/404', 'delay/0.1', 'delay/0.5', 'delay/1.0', 'status/500'];
          let endpointIndex = 0;
          const timer = setInterval(() => {
            if (activeScenario !== 'portscan') {
              clearInterval(timer);
              return;
            }
            // Send to different endpoints rapidly to simulate port scanning
            for (let i = 0; i < 15; i++) {
              const endpoint = endpoints[(endpointIndex + i) % endpoints.length];
              const timeoutId = setTimeout(() => {
                if (activeScenario === 'portscan') {
                  send(endpoint);
                }
              }, i * 5);
              timeouts.push(timeoutId);
            }
            endpointIndex = (endpointIndex + 1) % endpoints.length;
            log(`Port scan batch sent (15 requests to different endpoints)`, 'warning');
          }, 200); // Every 200ms
          timers.push(timer);
        };
```

