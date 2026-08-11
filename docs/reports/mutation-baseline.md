# 首轮 Mutation Baseline（2026-08-11）

## 方法

在提交 `829e910` 的独立临时 worktree 中逐个施加受控局部变异，每次只运行来源契约对应的测试。构建失败不计 killed；本轮四个变异均能构建，并因断言失败被杀死。临时 worktree 已移除。

| Mutant | 行为变化 | 捕获测试 | 结果 |
| --- | --- | --- | --- |
| error-dispatch | BusinessError 错误分派不再进入 BusinessEvent | `TestEventFromErrorDispatchesByKind` | killed |
| masker-suffix | 删除 `.token` / `_token` 等后缀敏感键匹配 | `TestFieldMaskerRecursivelyMasksStructuredValues` | killed |
| high-value-sampling | failed/error/blocked/denied 不再强制保留 | `TestResultKeepSamplerHighValue` | killed |
| field-ownership | Business ExtraAttrs 可以覆盖 canonical 字段 | `TestBusinessPayloadExtraAttrsCannotOverrideGovernanceKeys` | killed |

```text
mutation score = 4 / (4 + 0) = 100%
survival rate  = 0 / (4 + 0) = 0%
```

该分数只代表四个确定性高风险点，不代表整个仓库的 mutation coverage。后续扩展应继续记录 survived、no-coverage、timeout、build-error 与等价变异体，不得把构建失败计为 killed。
