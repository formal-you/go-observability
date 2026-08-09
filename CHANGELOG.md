# Changelog

本项目遵循 [语义化版本](https://semver.org/lang/zh-CN/)；**v0.x 允许破坏性变更**（键名、EventName、导出 API）。

本文件按版本汇总对使用方有影响的变更，不逐条复制 Git commit；具体提交记录以 `git log` 为准。尚未发布的多批次改动统一记录在 `[Unreleased]`。

## [Unreleased]

### Added

- 六类类型化事件（access / business / error / audit / security / probe）、`Logger` / `Writer` 接口与 JSONL / stdout / OTLP Writer
- `errs` 错误体系、错误到事件投影、Gin access / recover 中间件
- Trace / Metric / Log 三信号装配、资源属性、采样与环境变量出口选择
- `ResultKeepSampler`、`FieldMasker`；`example/nethttp`、`example/mall`、`example/metrics`
- LGTM 参考栈 `observability/`；Log / Trace / Metric / Error 管线模板 `observability/templates/`
- **开源文档集**：`CODE_OF_CONDUCT.md`、`SECURITY.md`、`SUPPORT.md`；`docs/{configuration,environment,security,README}.md`
- **example/config/**：`.env.example` / `app.example.yaml` / `collector.example.yaml` / compose 示意（**逐字段注释**）
- `example/README.md` 索引；templates 与 `otel-collector-config.yaml` 字段注释加强
- CI：fmt / vet / test / race / govulncheck；核心外部黑盒测试与各组件测试

### Changed

- **C1**: vendor 前缀 `mall.*` → `app.*`，Go 常量 `KeyApp*`
- **C2**: 领域 `business.*` 迁 `example/mall`；核心仅框架级 EventName
- **C3**: 官方中间件仅 Gin
- **C4**: 文档与注释以中文为主
- **B5**: Metric 使用方自建；**B9**: env 出口收敛
- OTel 属性键对齐 semconv 1.41.0；file / stdout 与 OTLP 采用双投影

### Fixed

- OTLP LogRecord 顶层字段映射：timestamp、severity、EventName 与 span context 不再重复写入属性
- 测试输出改用临时目录并正确关闭 Writer，避免在仓库内残留产物
- README 示例索引去重并补齐配置、文档与安全入口
