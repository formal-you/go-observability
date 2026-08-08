// Package file 提供 JSONL 文件 Writer：每行一个事件（扁平字段），便于本地排障与回放。
package file

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
)

// Writer 实现 log.Writer：把事件以 JSON 行追加写入文件。
type Writer struct {
	mu   sync.Mutex
	file *os.File
	enc  *json.Encoder
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
	return &Writer{file: f, enc: json.NewEncoder(f)}, nil
}

// Write 把事件写为一行 JSON（扁平字段 + msg=event_type）。
func (w *Writer) Write(_ context.Context, msg string, attrs ...slog.Attr) error {
	m := make(map[string]any, len(attrs)+1)
	m["msg"] = msg
	for _, a := range attrs {
		m[a.Key] = a.Value.Any()
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.enc.Encode(m)
}

// Close 关闭文件。
func (w *Writer) Close(_ context.Context) error { return w.file.Close() }
