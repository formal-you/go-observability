package stdout

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestWriterEmitsToOutput(t *testing.T) {
	var buf bytes.Buffer
	w, err := New(context.Background(), WithOutput(&buf))
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close(context.Background())
	if err := w.Write(context.Background(), "access"); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "access") {
		t.Errorf("输出应包含事件类型: %s", out)
	}
}
