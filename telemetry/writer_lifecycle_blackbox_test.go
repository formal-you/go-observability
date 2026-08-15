package telemetry_test

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	obslog "github.com/formal-you/go-observability/log"
	"github.com/formal-you/go-observability/telemetry"
)

func TestRuntimeWriterLifecycleIsPublicAndIdempotent(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "events.jsonl")
	runtime, err := telemetry.NewFileRuntime(telemetry.Config{
		Resource: telemetry.ResourceConfig{ServiceName: "lifecycle-test"},
		Log:      telemetry.LogConfig{Output: telemetry.SignalOutputFile, FilePath: path},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Shutdown(ctx)
	writer, err := runtime.NewWriter(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := writer.Write(ctx, "business", slog.String(string(obslog.KeyEventName), "order.created")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(ctx); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(ctx); err != nil {
		t.Fatalf("second Close must be idempotent: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var record map[string]any
	if err := json.Unmarshal(data[:len(data)-1], &record); err != nil {
		t.Fatalf("JSONL record is invalid: %v", err)
	}
	if record["service.name"] != "lifecycle-test" {
		t.Fatalf("service.name = %v", record["service.name"])
	}
}

func TestRuntimeNoneWriterHasManagedLifecycle(t *testing.T) {
	runtime, err := telemetry.NewRuntime(context.Background(), telemetry.Config{
		Enabled:  true,
		Resource: telemetry.ResourceConfig{ServiceName: "none-lifecycle-test"},
		Trace:    telemetry.TraceConfig{Output: telemetry.SignalOutputNone},
		Metric:   telemetry.MetricConfig{Output: telemetry.SignalOutputNone},
		Log:      telemetry.LogConfig{Output: telemetry.SignalOutputNone},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = runtime.Shutdown(context.Background()) }()
	writer, err := runtime.NewWriter(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err := writer.Write(context.Background(), "probe"); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestMultiWriterClosesAllManagedChildrenAndKeepsPlainWritersCompatible(t *testing.T) {
	first := &lifecycleWriter{closeErr: errors.New("first")}
	second := &lifecycleWriter{closeErr: errors.New("second")}
	plain := plainWriter{}
	writer := obslog.NewMultiWriter(first, plain, second)
	if err := writer.Write(context.Background(), "business"); err != nil {
		t.Fatal(err)
	}
	err := writer.Close(context.Background())
	if !errors.Is(err, first.closeErr) || !errors.Is(err, second.closeErr) {
		t.Fatalf("Close error = %v, want both child errors", err)
	}
	if first.closeCalls != 1 || second.closeCalls != 1 {
		t.Fatalf("close calls = %d/%d, want 1/1", first.closeCalls, second.closeCalls)
	}
	if err := writer.Close(context.Background()); !errors.Is(err, first.closeErr) || !errors.Is(err, second.closeErr) {
		t.Fatalf("second Close must preserve the aggregate error: %v", err)
	}
}

type lifecycleWriter struct {
	closeErr   error
	closeCalls int
}

func (w *lifecycleWriter) Write(context.Context, string, ...slog.Attr) error { return nil }
func (w *lifecycleWriter) Close(context.Context) error {
	w.closeCalls++
	return w.closeErr
}

type plainWriter struct{}

func (plainWriter) Write(context.Context, string, ...slog.Attr) error { return nil }
