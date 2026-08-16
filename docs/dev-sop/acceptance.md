# 契约与验收

> 状态：draft，待讨论。真源：`docs/dev-sop/testing.md`（正式测试契约）与 `docs/gitops/issues/`（本地 issue 契约）。

## 原则

- 黑盒优先：需求预期由独立黑盒测试表达，实现代码不得反向定义预期；
- 验收可观察、可复现、可度量：每条对应一个可观察结果或命令；
- 顺序：契约/黑盒（红）→ 最小实现（绿）→ 白盒边界测试。

## DoD（完成定义）

- [ ] 契约与黑盒测试先于实现且为绿
- [ ] 文档 / 示例 / CHANGELOG / ADR 索引同步（单一真源）
- [ ] gofmt / go build / go vet / go test / git diff --check 全绿（资源允许加 -race）
- [ ] 工作区中的用户改动未被混入本次提交
- [ ] 显式路径暂存 + Conventional Commit；不 push（除非用户明确要求）

## 回归与漂移

- 核心验收随迭代更像回归：需求变化时先更新契约与黑盒，再动实现；
- 文档漂移检测：新增决策必须进 ADR + 索引 + 路由表同步，避免真源分裂。

## 待讨论

- [ ] 验收清单是否落地为模板（[templates/acceptance-checklist.md](templates/acceptance-checklist.md)）并按任务勾选
- [ ] 是否引入 mutation / 性能基线证据（现状见 `docs/reports/`）