package log

import (
	"context"
	"errors"
	"log/slog"
)

// MultiWriter 把同一事件依次写给多个 Writer 的组合 Writer（装饰器）。
// 适用于 stdout + 文件 + OTLP 同时输出的场景；任一 Writer 失败不阻断其余 Writer，
// 最终返回 errors.Join 聚合的错误，由 Logger 交给 ErrorHandler 观察。
// 写入按传入顺序串行执行，保证确定性；并发语义由各 Writer 自行保证。
type MultiWriter []Writer

// NewMultiWriter 构造 MultiWriter；writers 为空时 panic（配置错误应尽早暴露），
// nil 项在写入时被跳过。
func NewMultiWriter(writers ...Writer) Writer {
	if len(writers) == 0 {
		panic("log: MultiWriter 至少需要一个 Writer")
	}
	return MultiWriter(writers)
}

// Write 依次写出到全部 Writer：单个失败继续其余，最后聚合所有错误。
func (m MultiWriter) Write(ctx context.Context, msg string, attrs ...slog.Attr) error {
	var errs []error
	for _, w := range m {
		if w == nil {
			continue
		}
		if err := w.Write(ctx, msg, attrs...); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
