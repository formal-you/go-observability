# 贡献指南

## 开发

1. 读 [AGENTS.md](AGENTS.md)（防漂移硬规则）。
2. Go **1.25+**（见 `go.mod`）。
3. 改代码后：

```bash
gofmt -w .
go vet ./...
go test ./...
```

低内存环境可设 `GOMAXPROCS=1`、`GOGC=30`。

## 提交

- Conventional Commits：`feat:` / `fix:` / `docs:` / `refactor!:` 等。
- schema / 键名变更必须同步 README 与相关文档，并更新 CHANGELOG `[Unreleased]`。
- 领域 business 事件不要加进核心 `types.go`（放接入方或 `example/`）。

## PR

- 附 `go test ./...` 通过说明。
- 破坏性变更在标题加 `!` 并写清迁移方式。
