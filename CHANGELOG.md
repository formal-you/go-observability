# Changelog

本项目遵循 [语义化版本](https://semver.org/lang/zh-CN/)；**v0.x 允许破坏性变更**（键名、EventName、导出 API）。

## [Unreleased]

### Changed

- **C1**: vendor 前缀 `mall.*` → `app.*`，Go 常量 `KeyApp*`
- **C2**: 领域 `business.*` 事件与专属键迁至 `example/mall`；核心仅框架级 EventName
- **C3**: 明确 Gin 中间件为可选集成；新增 `example/nethttp`
- **C4**: 导出注释与文档以中文为主（面向中文社区）
- **C5**: CI（fmt/vet/test/race/govulncheck）、CHANGELOG、CONTRIBUTING
- **C6**: 默认 `ResultKeepSampler`、`FieldMasker`
- **B5**: Metric 由使用方 `Providers.Meter()` 自建；`example/metrics` + templates
- **B9**: `SetupFromEnvironment` + `NewLogWriter`

### Added

- LGTM 参考栈 `observability/`
- 配置模板 `observability/templates/`
