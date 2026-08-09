# go-observability

面向 Go 服务的语义化日志与 OpenTelemetry Trace、Metric、Log 三信号装配库。文档以中文为主，字段尽量遵循 OpenTelemetry Semantic Conventions。

[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![CI](https://github.com/formal-you/go-observability/actions/workflows/ci.yml/badge.svg)](https://github.com/formal-you/go-observability/actions/workflows/ci.yml)

> 公开预览中：仓库已经公开，但尚未创建首个版本标签；首次发布前不承诺稳定 API、`@latest` 安装体验或 pkg.go.dev 已完成索引。正式发布将按 [发布检查清单](docs/release-checklist.md) 完成外部验证。

| 项目 | 当前约定 |
| --- | --- |
| Go | 1.25+（以 `go.mod` 为准） |
| 版本 | v0.x 期间公共 API 仍可能调整，见 [CHANGELOG](CHANGELOG.md) |
| 许可证 | MIT |

## 能做什么

- 六类类型化事件：access、business、error、audit、security、probe
- JSONL 文件、标准输出与 OTLP 日志 Writer
- 可注入的采样、脱敏和写入错误回调
- 公开的 [`telemetry`](telemetry/) 包：装配 Trace、Metric、Log Provider 并统一关闭
- Gin access/recover 中间件；标准库 `net/http` 接入示例
- 可选的本地 OpenTelemetry Collector、Loki、Tempo、Mimir、Grafana 参考栈

## 最小 JSONL 示例

完整程序位于 [`example/minimal/main.go`](example/minimal/main.go)，从仓库根目录执行 `go run ./example/minimal`，输出位于 `logs/events.jsonl`。发布标签就绪后，可在你自己的模块中按届时公布的版本安装。

```go
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	log "github.com/formal-you/go-observability"
	"github.com/formal-you/go-observability/writer/file"
)

func main() {
	ctx := context.Background()
	w, err := file.New("logs/events.jsonl")
	if err != nil {
		fmt.Fprintln(os.Stderr, "create log writer:", err)
		os.Exit(1)
	}

	writeFailed := false
	logger := log.NewLogger(w, log.WithErrorHandler(func(_ context.Context, _ string, _ []slog.Attr, err error) {
		writeFailed = true
		fmt.Fprintln(os.Stderr, "write event:", err)
	}))
	logger.Emit(ctx, log.BusinessEvent{
		EventMetadata: log.EventMetadata{Level: log.LevelInfo},
		Data: log.BusinessPayload{
			EventName: log.NewEventName("business", "order", "paid"),
			Result:    log.ResultSuccess,
		},
	})

	if err := w.Close(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "close log writer:", err)
		os.Exit(1)
	}
	if writeFailed {
		os.Exit(1)
	}
}
```

`Logger.Emit` 不返回 Writer 错误；生产接入必须配置 `WithErrorHandler`，并监控降级输出或丢日志风险。

## 进阶接入

- Gin：[`example/main.go`](example/main.go) 与 [`middleware/ginlog`](middleware/ginlog/)
- 标准库 HTTP：[`example/nethttp`](example/nethttp/)
- OTLP 与三信号：[`telemetry`](telemetry/) 和 [配置指南](docs/configuration.md)
- 本地 LGTM 栈：[`observability`](observability/)
- 指标：[`example/metrics`](example/metrics/)
- 领域事件注册：[`example/mall`](example/mall/)

从仓库根目录运行示例：

```powershell
$env:OTEL_SDK_DISABLED = "true"
go run ./example
Get-Content .\logs\events.jsonl
```

```bash
OTEL_SDK_DISABLED=true go run ./example
tail -n 1 ./logs/events.jsonl
```

设置 `OTEL_EXPORTER_OTLP_ENDPOINT` 后，主示例改走 OTLP；未设置时写入仓库根目录下的 `logs/events.jsonl`。完整命令见 [示例索引](example/README.md)。

## 包布局

```text
根包 log                 事件、Logger、Sampler、Masker
errs                     错误分类与事件投影
middleware/ginlog        Gin access 日志
middleware/recover       Gin panic 恢复
writer/file              JSONL 文件 Writer
writer/stdout            标准输出 Writer
writer/otlp              OTLP 日志 Writer
telemetry                三信号 Provider 装配
example                  可运行示例
observability            本地参考栈与管线模板
docs                     用户与贡献文档
```

## 采样与安全边界

- `TraceSampleRatio` 默认 `0.1`，这是头部采样。未被 SDK 导出的 trace 不会到达 Collector，因此 Collector 的 `tail_sampling` 无法恢复它。
- 需要按错误、延迟做尾部采样时，SDK 侧应先导出完整 trace（通常设为 `1.0`），再由 Collector 决定保留比例；同时评估成本与隐私影响。
- `FieldMasker` 只提供通用键名脱敏，不替代业务 PII 清单、访问控制、保留期限和审计策略。详见 [安全指南](docs/security.md)。

## 文档与协作

- [文档索引](docs/README.md)
- [配置指南](docs/configuration.md)
- [架构说明](docs/architecture.md)
- [贡献指南](CONTRIBUTING.md)
- [安全政策](SECURITY.md)
- [支持渠道](SUPPORT.md)

## 本地验证

```bash
go test ./...
go vet ./...
```

## License

MIT，见 [LICENSE](LICENSE)。
