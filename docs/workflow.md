# 开发工作流

## 变更步骤

1. 用 Issue 或变更说明写清用户问题、兼容性影响和验收条件。
2. 按 [正式测试契约](testing.md) 先补独立黑盒测试，再做最小实现与白盒边界测试。
3. 更新受影响的 README、配置样例和 CHANGELOG。
4. 运行格式化、静态检查、测试和差异检查。
5. 按可独立说明的批次提交 Git commit；多批次工作应保留多次提交记录。

```powershell
gofmt -w <modified-go-files>
$env:GOMAXPROCS=1
$env:GOGC=30
go build ./...
go vet ./...
go test ./...
git diff --check
```

资源允许时额外运行 `go test -race ./...`。`--help` 或仅 build 不能替代相关测试与全量测试。

## 兼容性规则

- 事件名、字段键、枚举值和公共 Go API 都属于使用方契约。
- v0.x 可以发生破坏性变更，但必须在 CHANGELOG 标明并提供迁移说明。
- 核心包不接收特定业务领域的事件常量；领域扩展使用 `BusinessPayload.ExtraAttrs`。
- 文档、示例与测试必须只依赖仓库内公开资料，不引用个人电脑路径或未发布设计文档。

## 提交前检查

- 没有密钥、真实用户数据、绝对路径、临时 JSONL 或编辑器文件。
- README 示例可编译，命令注明运行目录并同时照顾 PowerShell 与 bash。
- 新增链接能解析到仓库内文件；外部服务尚未上线时明确写“发布准备中”。
- Writer、Provider 等资源在示例和测试中被正确关闭。

## Git 门禁

- 每次完成的变更都必须创建 commit；一次提交聚焦一个可说明的逻辑变更。
- 使用 `git add <explicit-paths>` 显式暂存本次文件，禁止 `git add .` / `git add -A`，避免混入用户改动。
- 提交信息使用 Conventional Commits（`feat:` / `fix:` / `docs:` / `refactor:`）。
- 不 push、不 force push，除非用户明确要求。
