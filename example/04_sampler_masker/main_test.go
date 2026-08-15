package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRunKeepsOnlyGovernedEvents 验证示例自身承诺的可观察行为：
// EventTypeKeepSampler 只保留 error/security/audit 三类事件，
// 并且 app.secret 在写入前已被 FieldMasker 脱敏。
func TestRunKeepsOnlyGovernedEvents(t *testing.T) {
	path := filepath.Join(t.TempDir(), "governance.jsonl")
	if err := run(context.Background(), path); err != nil {
		t.Fatalf("run() error = %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 3 {
		t.Fatalf("output lines = %d, want 3\n%s", len(lines), data)
	}

	for _, want := range []string{`"type":"error"`, `"type":"security"`, `"type":"audit"`} {
		if !strings.Contains(string(data), want) {
			t.Errorf("output missing %s\n%s", want, data)
		}
	}
	if strings.Contains(string(data), `"app.secret":"should-be-redacted"`) {
		t.Fatalf("app.secret was not masked\n%s", data)
	}
	if !strings.Contains(string(data), `"app.secret":"***"`) {
		t.Errorf("masked app.secret not found\n%s", data)
	}
}
