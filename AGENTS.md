# AGENTS.md - go-observability agent entrypoint

本文件只保留所有变更都必须知道的入口与硬门禁。按任务触发读取链接文档；不要凭本文件摘要替代对应真源。

## 项目身份

- 模块：`github.com/formal-you/go-observability`，Go 1.26。
- 定位：基于 OpenTelemetry Semantic Conventions 1.41.0 的语义化日志组件，采用 OTel 键 + `app.*` vendor 命名空间。

## 按任务读取

- **任何代码或文档变更**：先读 [GitOps SOP](docs/gitops/gitops-sop.md)，其中包含验证、文档同步和 Git 提交门禁。
- **创建、重命名分支或调整分支治理**：先读 [分支命名规范](docs/gitops/branching.md)。
- **任何 Go 实现、公共 API、并发或代码注释变更**：先读 [代码与注释准则](docs/reference/coding-standards.md)；中文叙述保留标准英文术语，并明确资源所有权与并发契约。
- **需求、缺陷、公共行为或测试变更**：先读 [正式测试契约](docs/dev-sop/testing.md)；独立黑盒测试是需求验收依据。
- **重大架构或公共 API 变更（新增/更新 ADR）**：先读 [ADR 约定](docs/adr/README.md) 与 [GitOps SOP](docs/gitops/gitops-sop.md) 的变更步骤。
- **事件名、错误字段、采样或术语变更**：先读 [领域术语](CONTEXT.md) 与相关 [ADR](docs/adr/README.md)；EventName / ErrorCode 本次决策见 [ADR-0009](docs/adr/events/0009-event-name-fact-and-error-code.md)。
- **包边界、Writer、OTel 映射或中间件变更**：先读 [架构说明](docs/reference/architecture.md) 与 [OTel Logs 映射](docs/reference/otel-logs-data-model.md)。
- **telemetry、file-only、环境变量或部署配置变更**：先读 [配置指南](docs/reference/configuration.md) 与 [环境变量](docs/reference/environment.md)。
- **脱敏、PII、鉴权、安全或审计变更**：先读 [安全指南](docs/security.md)。
- **开始一个开发任务（含多 Agent 编排）**：先读 [开发流程体系](docs/dev-sop/README.md)；masterAgent=项目经理/产品经理/架构师，subAgent=探索/开发核心；提交与 PR 环节仍以 [GitOps SOP](docs/gitops/gitops-sop.md) 为准。

完整文档路由见 [docs/README.md](docs/README.md)，项目术语冲突时以 [CONTEXT.md](CONTEXT.md) 为准。

## 永久门禁

1. **黑盒优先**：需求预期由明确契约与独立黑盒测试表达；实现代码、内部结构和白盒单元测试不得反向定义预期。公共行为变化必须先更新黑盒验收与契约文档，再完成实现。
2. **核心零依赖**：`log/` 只依赖标准库；允许的依赖方向与 OTel 边界以 [架构说明](docs/reference/architecture.md) 为准。
3. **单一真源**：schema、键名、字段归属或公共 API 变化必须同步相关文档、示例、测试与 `CHANGELOG.md`，并进入同一提交。
4. **能力归库**：通用错误投影、HTTP/gRPC/Gin 收口和三信号装配在库内实现；应用差异通过公开配置或注入点表达。
5. **完成即提交**：按 [GitOps SOP](docs/gitops/gitops-sop.md) 完成全量门禁后，用显式路径暂存并创建 Conventional Commit；不 push，除非用户明确要求。
