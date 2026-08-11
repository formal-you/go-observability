// Command blackbox 生成覆盖 access / business / error 三类事件的 JSON 日志样例。
//
// 用途：黑盒验证——直接查看真实输出结构，防止实现漂移；配套 blackbox_test.go
// 对同一批事件逐行断言 JSON 结构（人工审核 + 自动化防漂移双保险）。
//
// 运行：go run ./example/blackbox
// 输出：example/blackbox/sample.jsonl（每行一个 JSON 对象；*.jsonl 已被 .gitignore
// 忽略，重新运行即重新生成，便于对照审核）。
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	log "github.com/formal-you/go-observability/log"
	"github.com/formal-you/go-observability/writer/file"
)

func main() {
	ctx := context.Background()
	path := filepath.Join("example", "blackbox", "sample.jsonl")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "mkdir:", err)
		os.Exit(1)
	}
	w, err := file.New(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "create writer:", err)
		os.Exit(1)
	}
	emitAll(ctx, log.NewLogger(w))
	if err := w.Close(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "close writer:", err)
		os.Exit(1)
	}
	fmt.Println("written:", path)
}
