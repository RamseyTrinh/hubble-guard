# Mô tả Cơ chế Đăng ký Luật (Rules) trong Hệ thống

## 3.2. Cơ chế Đăng ký và Quản lý Luật Phát hiện Anomaly

Hệ thống phát hiện anomaly sử dụng kiến trúc **Rule Engine** để quản lý và thực thi các luật phát hiện bất thường. Engine này được thiết kế theo mô hình **Plugin Architecture**, cho phép đăng ký động các luật mà không cần thay đổi code cốt lõi của hệ thống.

### 3.2.1. Kiến trúc Rule Engine

**Rule Engine** là thành phần trung tâm quản lý tất cả các luật phát hiện anomaly trong hệ thống. Engine được khởi tạo thông qua hàm `NewEngine()` và quản lý các thành phần sau:

- **Danh sách luật (Rules)**: Mảng các đối tượng implement interface `RuleInterface`, mỗi luật đại diện cho một loại anomaly cần phát hiện
- **Danh sách notifiers**: Các kênh thông báo (Telegram, Log, Webhook) để gửi cảnh báo khi phát hiện anomaly
- **Alert Channel**: Kênh giao tiếp (channel) với buffer 100 alerts để truyền cảnh báo giữa các thành phần
- **Mutex**: Cơ chế đồng bộ hóa thread-safe để đảm bảo an toàn khi truy cập đồng thời

```go
type Engine struct {
    rules          []RuleInterface
    alertNotifiers []NotifierInterface
    logger         *logrus.Logger
    mu             sync.RWMutex
    alertChannel   chan model.Alert
}
```

### 3.2.2. Quy trình Đăng ký Luật

Quy trình đăng ký luật được thực hiện trong hàm `RegisterBuiltinRulesFromYAML()` với các bước như sau:

**Bước 1: Đọc cấu hình từ file YAML**
Hệ thống đọc danh sách các luật từ file cấu hình `anomaly_detection.yaml`. Mỗi luật được định nghĩa với các thuộc tính:
- `name`: Tên loại luật (ví dụ: `traffic_spike`, `port_scan`, `block_connection`)
- `enabled`: Trạng thái kích hoạt (true/false)
- `severity`: Mức độ nghiêm trọng (CRITICAL, HIGH, MEDIUM, LOW)
- `thresholds`: Các ngưỡng phát hiện (multiplier, per_minute, distinct_ports, ...)

**Bước 2: Khởi tạo đối tượng luật**
Dựa trên tên luật, hệ thống tạo đối tượng luật tương ứng. Ví dụ với luật `traffic_spike`:

```go
promRule := builtin.NewDDoSRulePrometheus(
    ruleConfig.Enabled, 
    ruleConfig.Severity, 
    threshold, 
    promClient, 
    logger
)
```

Mỗi luật được khởi tạo với:
- Trạng thái enabled/disabled
- Mức độ nghiêm trọng
- Ngưỡng phát hiện (threshold)
- Prometheus client để truy vấn metrics
- Logger để ghi log

**Bước 3: Cấu hình luật**
Sau khi khởi tạo, luật được cấu hình thêm:
- **SetNamespaces()**: Thiết lập danh sách namespace cần giám sát
- **SetAlertEmitter()**: Thiết lập callback function để phát cảnh báo. Callback này sẽ gọi `engine.EmitAlert()` khi phát hiện anomaly

```go
promRule.SetNamespaces(yamlConfig.Namespaces)
promRule.SetAlertEmitter(func(alert *model.Alert) {
    engine.EmitAlert(*alert)
})
```

**Bước 4: Đăng ký luật vào Engine**
Luật được đăng ký vào Engine thông qua phương thức `RegisterRule()`. Phương thức này:
- Sử dụng mutex để đảm bảo thread-safe
- Thêm luật vào danh sách `rules`
- Ghi log thông tin đăng ký

```go
engine.RegisterRule(promRule)
```

**Bước 5: Khởi động luật**
Mỗi luật chạy trong một goroutine riêng biệt, thực hiện truy vấn Prometheus định kỳ:

```go
ctx := context.Background()
go promRule.Start(ctx)
```

Phương thức `Start()` của mỗi luật sẽ:
- Tạo ticker với interval mặc định (thường là 10 giây)
- Trong mỗi chu kỳ, truy vấn Prometheus để lấy metrics
- Phân tích metrics và so sánh với ngưỡng
- Nếu phát hiện anomaly, gọi `alertEmitter()` để phát cảnh báo

### 3.2.3. Cơ chế Phát Cảnh báo (Alert Emission)

Khi một luật phát hiện anomaly, nó gọi callback `alertEmitter` đã được đăng ký. Callback này sẽ gọi `engine.EmitAlert()` với các bước xử lý:

1. **Gửi vào Alert Channel**: Alert được đưa vào channel với buffer 100. Nếu channel đầy, alert sẽ bị bỏ qua và ghi log lỗi.

2. **Gửi đến Notifiers**: Engine lặp qua tất cả các notifiers đã đăng ký (Telegram, Log, Webhook) và gửi alert đến từng notifier. Nếu có lỗi, hệ thống ghi log nhưng không dừng quá trình.

```go
func (e *Engine) EmitAlert(alert model.Alert) {
    // Gửi vào channel
    select {
    case e.alertChannel <- alert:
    default:
        e.logger.Error("Alert channel is full, dropping alert")
    }
    
    // Gửi đến notifiers
    for _, notifier := range notifiers {
        if err := notifier.SendAlert(alert); err != nil {
            e.logger.Errorf("Failed to send alert: %v", err)
        }
    }
}
```

### 3.2.4. Ưu điểm của Kiến trúc này

1. **Tách biệt trách nhiệm**: Mỗi luật độc lập, tự quản lý logic phát hiện của mình
2. **Dễ mở rộng**: Thêm luật mới chỉ cần implement `RuleInterface` và đăng ký vào Engine
3. **Thread-safe**: Sử dụng mutex và channel để đảm bảo an toàn đa luồng
4. **Linh hoạt**: Có thể bật/tắt từng luật qua cấu hình mà không cần restart
5. **Tập trung xử lý alert**: Tất cả alerts được xử lý tập trung qua Engine, dễ quản lý và theo dõi

### 3.2.5. Ví dụ Đăng ký Luật Traffic Spike

Dưới đây là ví dụ minh họa quy trình đăng ký luật phát hiện traffic spike:

```go
// 1. Đọc cấu hình
ruleConfig := yamlConfig.Rules["traffic_spike"]
threshold := ruleConfig.Thresholds["multiplier"] // Ví dụ: 3.0

// 2. Khởi tạo luật
promRule := builtin.NewDDoSRulePrometheus(
    true,           // enabled
    "HIGH",         // severity
    3.0,            // threshold: 3x baseline
    promClient,     // Prometheus client
    logger          // logger
)

// 3. Cấu hình
promRule.SetNamespaces([]string{"default", "production"})
promRule.SetAlertEmitter(func(alert *model.Alert) {
    engine.EmitAlert(*alert)
})

// 4. Đăng ký
engine.RegisterRule(promRule)

// 5. Khởi động
go promRule.Start(context.Background())
```

Luật này sẽ chạy trong goroutine riêng, mỗi 10 giây truy vấn Prometheus để lấy rate của `hubble_flows_total`, so sánh với baseline, và phát cảnh báo nếu vượt quá 3 lần baseline.

---

*Tài liệu này mô tả cơ chế đăng ký luật trong hệ thống phát hiện anomaly dựa trên Prometheus metrics.*

