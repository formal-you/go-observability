package file

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriterAppendsJSONLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "logs", "events.jsonl")
	w, err := New(path)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close(context.Background())

	if err := w.Write(context.Background(), "access", slog.String("http.request.method", "GET"), slog.String("app.result", "success")); err != nil {
		t.Fatal(err)
	}
	if err := w.Write(context.Background(), "business", slog.String("event.name", "order.payment.succeeded")); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Fatalf("行数 = %d, want 2", len(lines))
	}
	if !strings.Contains(lines[0], `"http.request.method":"GET"`) || !strings.Contains(lines[0], `"type":"access"`) {
		t.Errorf("line0 = %s", lines[0])
	}
	if !strings.Contains(lines[1], `"event.name":"order.payment.succeeded"`) {
		t.Errorf("line1 = %s", lines[1])
	}
}

// TestWriteCanonicalOrder 验证 JSONL 行的固定字段顺序：
// level → type → 链路/延迟 → event.name → payload 字段 → app.result 收尾，
// 防止字段顺序退化成字典序或随构造顺序漂移。
func TestWriteCanonicalOrder(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	w, err := New(path)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close(context.Background())

	// 模拟一次 access 事件的扁平 attrs（normalize 输出顺序：metadata → payload → app.result）。
	if err := w.Write(context.Background(), "access",
		slog.String("level", "INFO"),
		slog.String("trace_id", "949058d5c20153624d52da3358038026"),
		slog.String("span_id", "bba473e9b3034af6"),
		slog.String("request_id", "req-1001"),
		slog.Int64("latency_ms", 12),
		slog.String("event.name", "http.request.completed"),
		slog.String("http.request.method", "GET"),
		slog.String("url.path", "/api/v1/products/42"),
		slog.Int("http.response.status_code", 200),
		slog.String("app.result", "success"),
	); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	line := strings.TrimSpace(string(data))
	var record map[string]any
	if err := json.Unmarshal([]byte(line), &record); err != nil {
		t.Fatalf("JSONL 不可解析: %v", err)
	}
	if _, ok := record["timestamp"]; !ok {
		t.Fatal("Writer 应为每条 JSONL 补齐 timestamp")
	}
	for _, key := range []string{"trace_id", "span_id", "request_id", "event.name", "http.request.method", "url.path", "http.response.status_code", "app.result"} {
		if _, ok := record[key]; !ok {
			t.Errorf("缺少字段 %s: %s", key, line)
		}
	}
}

func TestWriterResourceMetadataProtectedAndPerLine(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	w, err := New(path, WithResourceMetadata(ResourceMetadata{
		ServiceName: "mall-monolith", ServiceVersion: "1.0.0", ServiceInstanceID: "shop-01", DeploymentEnvironmentName: "production",
	}))
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close(context.Background())
	attrs := []slog.Attr{
		slog.String("service.name", "forged"),
		slog.String("deployment.environment.name", "forged"),
	}
	if err := w.Write(context.Background(), "order.payment.succeeded", attrs...); err != nil {
		t.Fatal(err)
	}
	if err := w.Write(context.Background(), "error.database.timeout"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		var record map[string]any
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			t.Fatalf("每行必须独立 JSON: %v", err)
		}
		if record["service.name"] != "mall-monolith" || record["deployment.environment.name"] != "production" {
			t.Fatalf("服务元数据未保护: %v", record)
		}
		if record["service.version"] != "1.0.0" || record["service.instance.id"] != "shop-01" {
			t.Fatalf("可选服务元数据缺失: %v", record)
		}
		if _, ok := record["timestamp"]; !ok {
			t.Fatalf("timestamp 缺失: %v", record)
		}
	}
}

func TestWriterOmitsEmptyOptionalMetadata(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	w, err := New(path, WithResourceMetadata(ResourceMetadata{
		ServiceName: "mall-monolith", DeploymentEnvironmentName: "development",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Write(context.Background(), "business", slog.String("service.version", "forged")); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var record map[string]any
	if err := json.Unmarshal(data, &record); err != nil {
		t.Fatal(err)
	}
	if _, ok := record["service.version"]; ok {
		t.Fatalf("空的进程级 service.version 应省略且不能被事件伪造: %v", record)
	}
	if _, ok := record["service.instance.id"]; ok {
		t.Fatalf("空 service.instance.id 应省略: %v", record)
	}
}

func TestWriterRotation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	w, err := New(path, WithRotation(RotationConfig{
		MaxSizeMB:  1,
		MaxBackups: 2,
		MaxAgeDays: 1,
	}))
	if err != nil {
		t.Fatal(err)
	}
	payload := strings.Repeat("x", 600*1024)
	for range 2 {
		if err := w.Write(context.Background(), "business", slog.String("app.payload", payload)); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	backups, err := filepath.Glob(filepath.Join(filepath.Dir(path), "events-*.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if len(backups) != 1 {
		t.Fatalf("轮转备份数=%d, want 1: %v", len(backups), backups)
	}
}

func TestWriterRejectsInvalidRotation(t *testing.T) {
	for _, config := range []RotationConfig{
		{},
		{MaxSizeMB: 1, MaxBackups: -1},
		{MaxSizeMB: 1, MaxAgeDays: -1},
	} {
		if _, err := New(filepath.Join(t.TempDir(), "events.jsonl"), WithRotation(config)); err == nil {
			t.Errorf("RotationConfig=%+v 应报错", config)
		}
	}
}
