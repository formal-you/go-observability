# ADR-0024：Gin 错误响应仅保留 ResponseProjector 扩展点

- 状态：Accepted
- 日期：2026-08-18
- 关联：ADR-0005（错误收口中间件抽取 httperr 契约核心 + 框架薄壳）

## 背景（Context）

`middleware/gin.ErrorConfig` 同时提供 `StatusForError` 与 `ResponseProjector`。
前者只覆盖 HTTP 状态码，后者已经同时决定 HTTP 状态码和响应体，导致 Gin 配置层
存在两个重叠的响应扩展入口。

`middleware/httperr` 核心层的 `StatusForError`、`ResponseBody` 与 `DefaultProjector`
仍然具有清晰的分层职责；本决策只调整 Gin 框架配置，不删除 httperr 核心能力。

此外，`middleware/http.ErrorConfig` 已经只暴露 `ResponseProjector`，Gin 与 net/http
的错误响应配置应保持相同的扩展模型。

## 决策（Decision）

1. 删除 `middleware/gin.ErrorConfig.StatusForError` 公共配置字段。
2. Gin 错误响应统一通过 `ResponseProjector httperr.Projector` 同时决定状态码和响应体。
3. 未提供 `ResponseProjector` 时，继续使用 `httperr.DefaultProjector`，保持默认响应行为不变。
4. 保留 `middleware/httperr.StatusForError`、`ResponseBody` 与 `DefaultProjector`，供核心默认组合、框架适配和自定义 projector 复用。
5. 这是一次公共 API 破坏性变更：使用 Gin `StatusForError` 的调用方必须迁移到 `ResponseProjector`。

迁移示例：

```go
ResponseProjector: func(err error, requestID string) (int, any) {
    return customStatusForError(err), httperr.ResponseBody(err, requestID)
},
```

如果调用方只需要默认响应体而修改状态码，可以在应用侧保留上述小型适配函数；库不再同时维护两个框架配置入口。

## 结果（Consequences）

正面：

- Gin 与 net/http 的错误响应扩展模型一致；
- 状态码与响应体由一个 projector 原子决定，减少配置歧义；
- `ErrorResponse` 的默认路径更直接，不再在框架壳中拼装部分投影。

代价：

- 现有使用 `ErrorConfig.StatusForError` 的 Gin 接入方需要修改配置并重新编译；
- 旧字段不能通过兼容别名保留，否则会重新引入两个重叠入口；
- 既有默认状态码、默认响应体、错误事件和黑盒行为保持不变。

被否方案：

- **继续保留两个字段**：虽然可提供只改状态码的快捷方式，但会维持状态码与完整响应投影的重复配置面；
- **仅标记 `StatusForError` Deprecated**：适合需要跨版本渐进迁移的已发布 API，但当前决策选择直接收敛公共配置模型。
