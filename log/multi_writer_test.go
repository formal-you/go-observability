package log

import (
	"context"
	"errors"
	"log/slog"
	"testing"
)

var (
	errMultiB = errors.New("writer b failed")
	errMultiC = errors.New("writer c failed")
)

func TestNewMultiWriterPanicsEmpty(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("NewMultiWriter() 应 panic")
		}
	}()
	NewMultiWriter()
}

func TestMultiWriterWritesToAll(t *testing.T) {
	a := &captureWriter{}
	b := &captureWriter{}
	w := NewMultiWriter(a, b)
	if err := w.Write(context.Background(), "business", slog.String("app.result", "success")); err != nil {
		t.Fatalf("Write err = %v", err)
	}
	if len(a.msgs) != 1 || a.msgs[0] != "business" {
		t.Errorf("writer a msgs = %v, want [business]", a.msgs)
	}
	if len(b.msgs) != 1 || b.msgs[0] != "business" {
		t.Errorf("writer b msgs = %v, want [business]", b.msgs)
	}
}

func TestMultiWriterAggregatesErrors(t *testing.T) {
	a := &captureWriter{}
	b := &captureWriter{err: errMultiB}
	c := &captureWriter{err: errMultiC}
	w := NewMultiWriter(a, b, c)
	err := w.Write(context.Background(), "business")
	if err == nil {
		t.Fatal("应聚合 writer 错误")
	}
	if !errors.Is(err, errMultiB) || !errors.Is(err, errMultiC) {
		t.Fatalf("聚合错误应包含 b/c，实际: %v", err)
	}
	if len(a.msgs) != 1 {
		t.Errorf("失败 writer 不应阻断其余 writer，a.msgs = %v", a.msgs)
	}
}

func TestMultiWriterSkipsNil(t *testing.T) {
	a := &captureWriter{}
	w := NewMultiWriter(nil, a)
	if err := w.Write(context.Background(), "business"); err != nil {
		t.Fatalf("Write err = %v", err)
	}
	if len(a.msgs) != 1 {
		t.Errorf("nil writer 应被跳过，a.msgs = %v", a.msgs)
	}
}
