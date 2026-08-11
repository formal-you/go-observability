// Package file 提供 JSONL 文件 Writer：每行一个事件（扁平字段），便于本地排障与回放。
package file

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	"github.com/formal-you/go-observability/log"
)

// Writer 实现 log.Writer：把事件以 JSON 行追加写入文件。
type Writer struct {
	mu   sync.Mutex
	file *os.File
}

// New 创建文件 Writer；自动创建父目录，append 模式。
func New(path string) (*Writer, error) {
	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, err
	}
	return &Writer{file: f}, nil
}

// Write 把事件写为一行 JSON（扁平字段 + msg=event_type）。
// 字段按固定顺序输出，跨事件保持一致，便于人眼扫描与工具解析：
//
//	timestamp → level → msg → trace_id/span_id/request_id/latency_ms → event.name
//	→ 其余 payload 字段（按事件构造顺序）→ app.result 恒为最后。
//
// 键已存在时保留首次出现位置、以最后一次值为准（与 map 语义一致）。
func (w *Writer) Write(_ context.Context, msg string, attrs ...slog.Attr) error {
	line, err := marshalLine(msg, attrs)
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
func marshalLine(msg string, attrs []slog.Attr) ([]byte, error) {
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

	var buf bytes.Buffer
	buf.Grow(256)
	buf.WriteByte('{')
	first := true
	writeKV := func(key string, value any) error {
		kb, err := json.Marshal(key)
		if err != nil {
			return err
		}
		vb, err := json.Marshal(value)
		if err != nil {
			return err
		}
		if !first {
			buf.WriteByte(',')
		}
		first = false
		buf.Write(kb)
		buf.WriteByte(':')
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
	if err := writeKV("msg", msg); err != nil {
		return nil, err
	}
	for _, key := range order {
		if key == string(log.KeyTimestamp) || key == string(log.KeyLevel) || key == string(log.KeyAppResult) {
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
func (w *Writer) Close(_ context.Context) error { return w.file.Close() }
