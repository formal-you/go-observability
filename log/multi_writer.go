package log

import (
	"context"
	"errors"
	"log/slog"
	"sync"
)

// MultiWriter 把同一事件依次写给多个 Writer 的组合 Writer（装饰器）。
// 适用于 stdout + 文件 + OTLP 同时输出的场景；任一 Writer 失败不阻断其余 Writer，
// 最终返回 errors.Join 聚合的错误，由 Logger 交给 ErrorHandler 观察。
// 写入按传入顺序串行执行，保证确定性；并发语义由各 Writer 自行保证。
type MultiWriter []Writer

type managedWriter struct {
	Writer
	close func(context.Context) error
	once  sync.Once
	err   error
}

// ManageWriter 把任意 Writer 适配为 ManagedWriter。Writer 已实现 Close 时调用它，
// 否则 Close 为 no-op；无论哪种情况，Close 都只执行一次并在后续调用返回相同结果。
func ManageWriter(writer Writer) ManagedWriter {
	if writer == nil {
		panic("log: Writer 不能为空")
	}
	closeFn := func(context.Context) error { return nil }
	if closer, ok := writer.(interface {
		Close(context.Context) error
	}); ok {
		closeFn = closer.Close
	}
	return &managedWriter{Writer: writer, close: closeFn}
}

func (w *managedWriter) Close(ctx context.Context) error {
	w.once.Do(func() { w.err = w.close(ctx) })
	return w.err
}

// NewMultiWriter 构造 MultiWriter；writers 为空时 panic（配置错误应尽早暴露），
// nil 项在写入时被跳过。
func NewMultiWriter(writers ...Writer) ManagedWriter {
	if len(writers) == 0 {
		panic("log: MultiWriter 至少需要一个 Writer")
	}
	multi := MultiWriter(writers)
	return &managedWriter{
		Writer: multi,
		close: func(ctx context.Context) error {
			var errs []error
			for _, writer := range writers {
				if writer == nil {
					continue
				}
				if closer, ok := writer.(interface {
					Close(context.Context) error
				}); ok {
					errs = append(errs, closer.Close(ctx))
				}
			}
			return errors.Join(errs...)
		},
	}
}

// Write 依次写出到全部 Writer：单个失败继续其余，最后聚合所有错误。
func (m MultiWriter) Write(ctx context.Context, eventType string, attrs ...slog.Attr) error {
	var errs []error
	for _, w := range m {
		if w == nil {
			continue
		}
		if err := w.Write(ctx, eventType, attrs...); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
