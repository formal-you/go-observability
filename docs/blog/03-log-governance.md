# 03 · 日志治理：采样、脱敏、多出口

## 一、痛点

日志不是越多越好：

- 成功访问日志高频、低价值，会淹没错误信号；
- token、手机号、证件号藏在字段里，一旦落地就是风险；
- 本地排查要 JSONL，后端可观测要 OTLP，两套逻辑容易漂移。

`go-observability` 把这些治理动作变成显式注入点：`Sampler`、`Masker`、`MultiWriter`。

## 二、治理管线

```text
metadata 补全 -> 最低级别过滤 -> Masker -> Sampler -> Writer.Write
```

顺序很重要：脱敏先于采样，采样后事件才真正交给 Writer。

## 三、采样与脱敏

从仓库根目录运行：

```powershell
go run ./example/04_sampler_masker
Get-Content .\logs\governance.jsonl
```

```bash
go run ./example/04_sampler_masker
cat ./logs/governance.jsonl
```

`04_sampler_masker` 使用确定性的 `EventTypeKeepSampler`：

- 只保留 `error` / `security` / `audit`；
- `access` / `business` 被 Fallback 丢弃；
- `app.secret` 被 `FieldMasker` 替换为 `***`。

预期输出只有 3 条事件，其中 `error` 事件的 `app.secret` 已脱敏。

```go
log.NewLogger(w,
    log.WithMasker(log.FieldMasker{Keys: []string{"app.secret"}}),
    log.WithSampler(log.EventTypeKeepSampler{
        KeepTypes: []log.EventType{log.EventError, log.EventSecurity, log.EventAudit},
        Fallback:  log.SamplerFunc(func(context.Context, []slog.Attr) bool { return false }),
    }),
)
```

## 四、多出口

从仓库根目录运行：

```powershell
go run ./example/05_multiwriter
Get-Content .\logs\multiwriter.jsonl
```

```bash
go run ./example/05_multiwriter
cat ./logs/multiwriter.jsonl
```

同一个 `BusinessEvent` 会：

- 写一份扁平 JSONL 到文件，便于本地检索；
- 写一份 OTel LogRecord 到 stdout，便于后端消费。

`NewMultiWriter` 串行写入，任一 Writer 失败不阻断其余，最终用 `errors.Join` 聚合错误。

## 五、最佳实践

1. 高价值结果（`failed` / `error` / `blocked` / `denied`）必须保留；
2. 成功访问噪音显式采样，健康检查在入口确定性排除；
3. PII 键用 `Masker` 处理，不要等落到存储后再清洗；
4. 文件出口与 OTLP 出口用同一事件模型，不要写两套埋点。

下一篇：[04 · 三信号统一装配](04-telemetry-assembly.md)。
