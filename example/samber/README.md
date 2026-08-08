# samber 借鉴 spike（对照实验）

目的：验证把 go-observability 事件喂给 samber slog handler 链（slog-multi / slog-formatter / slog-sampling）的效果与兼容性，再决定是否替换现有 writer 层。

## 实验内容

- slog-multi：Pipe 中间件链 + Fanout（JSON stdout + 文件 samber-out.jsonl）。
- slog-formatter：PIIFormatter 掩码 mall.user_id。
- slog-sampling：UniformSamplingOption 采样（当前 Rate=1.0 全量，便于对照；<1 即真实采样）。
- 兼容性：我们的 EventPayload.Attrs() 输出 []slog.Attr，直接喂给 slog 记录。

## 运行

```powershell
go run ./example/samber
```

输出：stdout 打印 JSON，同时写入 samber-out.jsonl；对比 example/logs/events.jsonl（现有 writer 产物）。

## 结论（2026-08-08 spike 结果）

- 兼容性：EventPayload.Attrs() 喂给 slog 链可用；slog-multi Pipe/Fanout、slog-formatter、slog-sampling 均正常。
- 掩码：FormatByKey 无条件掩码生效（mall.user_id → ***）；PIIFormatter 只掩“长得像 PII”的值（邮箱/电话等），对自定义 key 不适用。
- 重复键：喂扁平 attrs 给 slog 会出现 level/timestamp 重复（slog 自带 + 我们的 attrs）；接入时需 strip 保留键，或只喂业务字段（这正是我们 normalize 层已做的事，samber 链需在外部重复一次）。
- 采样：UniformSamplingOption 中间件可用（Rate=1.0 全量；<1 真实采样）。
- 待决策：是否用 samber 链替换 writer/{otlp,stdout,file}（收益=现成生态；代价=额外依赖 + 键去重处理）。不合并回 main，由决策者确认。