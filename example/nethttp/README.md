# example/nethttp — 无 Gin 的 access 埋点（C3）

官方中间件：`middleware/ginlog`、`middleware/recover`（Gin 生态）。

标准库 `net/http` **不** 进核心依赖；使用方按本示例自行包装 `AccessEvent` 即可。

```bash
go run .
# curl http://127.0.0.1:8081/healthz
# 查看 logs/nethttp-events.jsonl
```
