// Package file 提供 JSONL 文件 Writer：每行一个事件（扁平字段），便于本地排障与回放。
package file

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/formal-you/go-observability/log"
	"gopkg.in/natefinch/lumberjack.v2"
)

// Writer 实现 log.Writer：把事件以 JSON 行追加写入文件。
type Writer struct {
	mu       sync.Mutex
	file     io.WriteCloser
	metadata ResourceMetadata
	rotation *RotationConfig
}

// ResourceMetadata 描述写入每条 JSONL 的进程级服务身份。
// 字段使用 OTel Resource 的规范键投影，空值不会写出。
type ResourceMetadata struct {
	ServiceName               string
	ServiceVersion            string
	ServiceInstanceID         string
	DeploymentEnvironmentName string
}

// RotationConfig 控制基于文件大小的日志轮转。MaxSizeMB 必须大于零；
// MaxBackups/MaxAgeDays 为零表示不按该维度删除旧文件；两者同时为零时无任何保留上限。
type RotationConfig struct {
	MaxSizeMB  int
	MaxBackups int
	MaxAgeDays int
	Compress   bool
	LocalTime  bool
}

// Option 配置文件 Writer。
type Option func(*Writer)

// WithResourceMetadata 让 Writer 在每条 JSONL 中注入不可被事件属性覆盖的服务身份。
func WithResourceMetadata(metadata ResourceMetadata) Option {
	return func(w *Writer) { w.metadata = metadata }
}

// WithRotation 启用文件轮转；轮转文件与当前文件位于同一目录。
func WithRotation(config RotationConfig) Option {
	return func(w *Writer) { w.rotation = &config }
}

// New 创建文件 Writer；自动创建父目录，append 模式。可选的 ResourceMetadata
// 会作为进程级字段写入每条记录；不传选项时保持旧行为。
func New(path string, opts ...Option) (*Writer, error) {
	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}
	}
	w := &Writer{}
	for _, opt := range opts {
		opt(w)
	}
	if w.rotation != nil {
		if err := validateRotation(*w.rotation); err != nil {
			return nil, err
		}
		if w.rotation.MaxBackups == 0 && w.rotation.MaxAgeDays == 0 {
			slog.Warn("file rotation has no retention limit", "path", path)
		}
		w.file = &lumberjack.Logger{
			Filename:   path,
			MaxSize:    w.rotation.MaxSizeMB,
			MaxBackups: w.rotation.MaxBackups,
			MaxAge:     w.rotation.MaxAgeDays,
			Compress:   w.rotation.Compress,
			LocalTime:  w.rotation.LocalTime,
		}
		return w, nil
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, err
	}
	w.file = f
	return w, nil
}

func validateRotation(config RotationConfig) error {
	if config.MaxSizeMB <= 0 {
		return fmt.Errorf("file: rotation max size must be positive")
	}
	if config.MaxBackups < 0 {
		return fmt.Errorf("file: rotation max backups must not be negative")
	}
	if config.MaxAgeDays < 0 {
		return fmt.Errorf("file: rotation max age must not be negative")
	}
	return nil
}

// Write 把事件写为一行 JSON（扁平字段 + type=event_type）。
// 字段按固定顺序输出，跨事件保持一致，便于人眼扫描与工具解析：
//
//	timestamp → level → type → trace_id/span_id/request_id/latency_ms → event.name
//	→ 其余 payload 字段（按事件构造顺序）→ app.result 恒为最后。
//
// 键已存在时保留首次出现位置、以最后一次值为准（与 map 语义一致）。
func (w *Writer) Write(_ context.Context, eventType string, attrs ...slog.Attr) error {
	line, err := marshalLine(eventType, attrs, w.metadata)
	if err != nil {
		return err
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	_, err = w.file.Write(line)
	return err
}

// marshalLine 按规范字段顺序序列化一行 JSON（含换行）。
// 任何值无法序列化时返回错误，不写出部分内容。
func marshalLine(eventType string, attrs []slog.Attr, metadata ResourceMetadata) ([]byte, error) {
	order := make([]string, 0, len(attrs)+1)
	values := make(map[string]any, len(attrs)+1)
	for _, a := range attrs {
		if a.Key == "" {
			continue
		}
		if _, ok := values[a.Key]; !ok {
			order = append(order, a.Key)
		}
		values[a.Key] = a.Value.Any()
	}
	timestamp, validTimestamp := values[string(log.KeyTimestamp)].(time.Time)
	if !validTimestamp || timestamp.IsZero() {
		values[string(log.KeyTimestamp)] = time.Now()
		if !validTimestamp {
			order = append([]string{string(log.KeyTimestamp)}, order...)
		}
	}
	metadataValues := []struct {
		key   string
		value string
	}{
		{"service.name", metadata.ServiceName},
		{"service.version", metadata.ServiceVersion},
		{"service.instance.id", metadata.ServiceInstanceID},
		{"deployment.environment.name", metadata.DeploymentEnvironmentName},
	}
	metadataKeys := make(map[string]struct{}, len(metadataValues))
	for _, item := range metadataValues {
		metadataKeys[item.key] = struct{}{}
		if item.value != "" {
			values[item.key] = item.value
		}
	}

	var buf bytes.Buffer
	buf.Grow(256)
	buf.WriteByte('{')
	first := true
	writeKV := func(key string, value any) error {
		if !first {
			buf.WriteByte(',')
		}
		first = false
		if jsonSafeString(key) {
			buf.WriteByte('"')
			buf.WriteString(key)
			buf.WriteByte('"')
		} else {
			kb, err := json.Marshal(key)
			if err != nil {
				return err
			}
			buf.Write(kb)
		}
		buf.WriteByte(':')
		if s, ok := value.(string); ok && jsonSafeString(s) {
			buf.WriteByte('"')
			buf.WriteString(s)
			buf.WriteByte('"')
			return nil
		}
		vb, err := json.Marshal(value)
		if err != nil {
			return err
		}
		buf.Write(vb)
		return nil
	}

	if v, ok := values[string(log.KeyTimestamp)]; ok {
		if err := writeKV(string(log.KeyTimestamp), v); err != nil {
			return nil, err
		}
	}
	if v, ok := values[string(log.KeyLevel)]; ok {
		if err := writeKV(string(log.KeyLevel), v); err != nil {
			return nil, err
		}
	}
	if err := writeKV("type", eventType); err != nil {
		return nil, err
	}
	for _, item := range metadataValues {
		if item.value != "" {
			if err := writeKV(item.key, values[item.key]); err != nil {
				return nil, err
			}
		}
	}
	for _, key := range order {
		if key == string(log.KeyTimestamp) || key == string(log.KeyLevel) || key == string(log.KeyAppResult) {
			continue
		}
		if _, protected := metadataKeys[key]; protected {
			continue
		}
		if err := writeKV(key, values[key]); err != nil {
			return nil, err
		}
	}
	if v, ok := values[string(log.KeyAppResult)]; ok {
		if err := writeKV(string(log.KeyAppResult), v); err != nil {
			return nil, err
		}
	}
	buf.WriteByte('}')
	buf.WriteByte('\n')
	return buf.Bytes(), nil
}

// Close 关闭文件。
func (w *Writer) Close(_ context.Context) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.file.Close()
}

// jsonSafeChar 报告 b 是否可直接写入 JSON 字符串字面量，且与 encoding/json.Marshal
// 的字符串编码逐字节一致：ASCII 可打印、非引号/反斜杠，并排除 < > &
// （encoding/json 默认对这三个字符做 HTML 转义，写成 \u003c 等）。
func jsonSafeChar(b byte) bool {
	return b >= 0x20 && b < 0x7f && b != '"' && b != '\\' && b != '<' && b != '>' && b != '&'
}

// jsonSafeString 判断 s 是否全部由 jsonSafeChar 组成；是则 writeKV 直接写出，
// 否则回退 json.Marshal，保证与既有 JSONL 输出逐字节一致。
func jsonSafeString(s string) bool {
	for i := 0; i < len(s); i++ {
		if !jsonSafeChar(s[i]) {
			return false
		}
	}
	return true
}
