// Command multiwriter 演示 log.NewMultiWriter 把同一个事件 fan-out 到多个 Writer。
//
// 运行方式（从仓库根目录）：
//
//	go run ./example/05_multiwriter
//	Get-Content .\logs\multiwriter.jsonl
//
// 教学要点：同一归一化事件在不同 Writer 里会有不同投影：
//   - file.Writer 输出扁平 JSONL，便于本地检索与回放；
//   - stdout.Writer 输出 OTel LogRecord，timestamp / level / trace 进入顶层字段。
//
// MultiWriter 按传入顺序串行写入，任一 Writer 失败不阻断其余，最终用 errors.Join 聚合错误。
package main

import (
	"context"
	"fmt"
	"os"

	log "github.com/formal-you/go-observability/log"
	"github.com/formal-you/go-observability/writer/file"
	"github.com/formal-you/go-observability/writer/stdout"
)

func main() {
	if err := run(context.Background(), "logs/multiwriter.jsonl"); err != nil {
		fmt.Fprintln(os.Stderr, "multiwriter:", err)
		os.Exit(1)
	}
}

// run 是示例的可测试主体，输出路径作为参数传入。
func run(ctx context.Context, path string) error {
	_ = os.Remove(path)

	// file.New 是纯 JSONL 文件 Writer，append 模式，自动创建父目录。
	fileWriter, err := file.New(path)
	if err != nil {
		return err
	}
	// stdout.New 基于 OTel stdout log exporter，默认写 os.Stdout。
	// 它和 file.Writer 实现同一个 log.Writer 接口，因此可以放进同一 MultiWriter。
	stdoutWriter, err := stdout.New(ctx)
	if err != nil {
		_ = fileWriter.Close(ctx)
		return err
	}

	// NewMultiWriter 返回 ManagedWriter：写入时按顺序 fan-out，关闭时
	// 尝试关闭所有实现了 Close 的子 Writer，并聚合关闭错误。
	w := log.NewMultiWriter(fileWriter, stdoutWriter)
	logger := log.NewLogger(w)

	logger.Emit(ctx, log.BusinessEvent{
		EventMetadata: log.EventMetadata{Level: log.LevelInfo},
		Data: log.BusinessPayload{
			EventName: log.NewEventName("order", "payment", "succeeded"),
			Result:    log.ResultSuccess,
		},
	})

	if err := w.Close(ctx); err != nil {
		return err
	}
	fmt.Println("written: logs/multiwriter.jsonl (stdout also received the same event)")
	return nil
}
