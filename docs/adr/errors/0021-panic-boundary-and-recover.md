# ADR-0021：Panic 只用于启动期契约校验，Recover 是请求期最后保险

- 状态：Accepted
- 日期：2026-08-16
- 关联：ADR-0010（严格错误构造）、ADR-0013（有界 StackPolicy）、ADR-0005（错误收口中间件）、ADR-0015（Error Registry）

## 背景（Context）

仓库里有两类 panic 容易混淆：一类在进程启动期由 `Must*` / `NewEventName` / nil Logger
触发，属于 fail-fast；另一类若出现在 handler 中，会被 `ginmw.Recover` 捕获后继续运行。
后者看似“不宕机”，实际把程序缺陷变成了带病运行：错误没有走 typed error 路径，
只能依赖 Recover 留下的堆栈事后排查。若没有单点规则，就会出现“启动期 panic 正确、
请求期 panic 也被容忍”的漂移。

## 决策（Decision）

- Panic 只用于进程启动期（依赖初始化完成后、开始接流量前）的程序员错误或配置/契约
  不变式：`MustRegisterErrorCode` / `MustRegisterErrorContract`、`log.NewEventName`、
  中间件必填 Logger 为空等。此时进程尚未服务，fail-fast 应尽早暴露编码错误。
- 请求路径禁止用 panic 表达业务/系统错误：预期内拒绝使用 `errs.BizError`
  （KindValidation / KindBusiness），非预期故障使用 `errs.SystemError`；两者经
  `ginmw.Abort` / error return 进入错误收口，渲染状态码并写 typed 事件。
- `Recover` 是最后一道保险，不是容错方案：只捕获真正的程序缺陷（nil 解引用、
  slice 越界等），转成 `runtime.panic.occurred` + `error.type=INTERNAL` 的
  `ErrorEvent`，并自动附带堆栈与代码位置（`errs.CaptureStack` / `errs.CaptureSource`），
  再返回统一 500，使单个坏请求不拖垮进程，同时留下可查询证据。
- panic 诊断优先使用该事件内的 `stacktrace` 与 `code.file.path` / `code.line.number`，
  并配合 trace/span/request_id；pprof 是持续剖析（CPU/内存/goroutine）工具，
  不作为 panic 现场的首要诊断手段。
- 线上应就 `runtime.panic.occurred` / `error.type=INTERNAL` 建告警；是否在 panic
  率超过阈值时 fail-stop（崩溃重启）由接入方运维策略决定，库不默认吞掉或放大。

## 被否方案

- 业务错误用 panic 抛给 Recover：被否，typed error 路径（状态码、error.code/type、
  采样）会被绕过，业务拒绝与程序缺陷混淆。
- 把 pprof 当作 panic 现场主诊断工具：被否，panic 事件已携带完整堆栈与代码位置，
  pprof 无法替代 `runtime/debug.Stack` 的触发点定位。
- Recover 吞掉 panic 不写事件：被否，失去 trace/span、堆栈与代码位置，线上问题不可追溯。

## 结果（Consequences）

- 启动期错误仍 fail-fast；请求期业务/系统错误走统一收口，不再经 panic 旁路。
- Recover 的定位从“兜住一切”收敛为“程序缺陷的最后保险 + 证据出口”。
- 日志侧可稳定告警 `runtime.panic.occurred` / `error.type=INTERNAL`；定位路径统一为
  ErrorEvent 的 stacktrace + code.*，pprof 作为补充剖析手段。
- 新代码 Review 时按“启动期 panic / 请求期 return error / Recover 兜底”三层检查，
  不再以“线上不宕机”为由接受请求期 panic。
