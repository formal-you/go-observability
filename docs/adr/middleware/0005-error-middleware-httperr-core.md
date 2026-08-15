# ADR 0005：错误收口中间件抽取 httperr 契约核心 + 框架薄壳

  状态：Accepted
  日期：2026 08 11
  关联：ADR 0001 / ADR 0002（错误模型决策）

## 背景（Context）

错误收口族存在三类重复实现：①「errs.Kind → HTTP 状态码/响应体」在 errresp / recover /
nethttp / kratos 四个包里各写一份；②「span context → EventMetadata」在五处重复；
③「panic → SystemError → 事件 → 响应」在 Gin 与 net/http 两处重复。同时分类不明显——
中间件按「框架×关注点」混合排布，没有框架无关的错误契约层。

## 决策（Decision）

  新增 `middleware/httperr`：框架无关的错误契约核心，不依赖任何 Web 框架，提供
  `Projector`（统一投影签名）、`StatusForError`（Kind→状态码）、`ClassifyError`
  （安全 reason/message/metadata）、`ResponseBody`（扁平响应体）、`DefaultProjector`、
  `EventMetadataFromContext`（span 元数据）、`SystemErrorFromPanic`（panic 构造）。
  errresp / recover / nethttp / kratos 收敛为 httperr 的框架薄壳，只做「框架错误挂载
  机制 → 本核心」的转换；`ginlog` / `metrics` / `trace` 不在本次范围。
  统一 `Config.ResponseProjector` 类型为 `httperr.Projector`（`(int, gin.H)` 合并为
  `(int, any)`）；`recover.Config.RequestID` 改名 `GetRequestID` 与 errresp/nethttp 对齐。
  panic 事件 `exception.message` 统一为 panic 值文本（net/http 原固定文案「internal
  server error」变更），由 `httperr.SystemErrorFromPanic` 收敛。

## 被否方案

  全量合并错误中间件（把显式错误 + panic + 传输映射合成一个包）：不同框架的错误挂载
  机制差异大（c.Errors / SetError / kratos encoder），合并会引入框架耦合，保持「核心
  契约 + 框架壳」更清晰。

## 结果（Consequences）

  正面：契约逻辑单点维护；框架壳只保留事件投影与响应渲染；新框架适配可直接复用 httperr。
  代价：破坏性变更——`ResponseProjector` 类型统一（mall 的 `(int, gin.H)` 需改
  `(int, any)`）、`recover.Config.RequestID` 改名；外部消费者 mall / ai gateway /
  kratos mall 同步适配。
  兼容：errresp / recover / nethttp / kratos 的公开函数名与主要 Config 字段保持；
  错误事件与响应契约行为不变（既有黑盒测试原样通过）。
