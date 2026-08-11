# 架构决策记录（ADR）

本目录以 Architecture Decision Records（ADR）形式记录本仓库的关键技术决策：
「为什么这么做」以及被否掉的备选方案，供后续 Review 与演进时追溯。

## 约定

- 命名：`NNNN-短横线标题.md`，编号递增、不重用、不删改历史。
- 模板：背景（Context）→ 决策（Decision）→ 结果（Consequences），采用 Nygard ADR 风格。
- 状态：Proposed / Accepted / Deprecated / Superseded（被取代时在正文标注关联）。
- 定位：ADR 只记决策与理由；术语以 `CONTEXT.md` 为准，代码行为以代码为准。
  按 AGENTS.md 防漂移精神，改代码与改 ADR 应在同一提交内完成。

## 索引

| 编号 | 标题 | 状态 | 日期 |
| --- | --- | --- | --- |
| [0001](0001-errorcode-three-segment-format.md) | ErrorCode 采用三段式（服务/模块）.（场景/操作）.（结果/具体错误） | Accepted | 2026-08-11 |
| [0002](0002-errortype-low-cardinality-domain-reason.md) | ErrorType 映射 OTel error.type，保持低基数，采用 domain.reason 格式 | Accepted | 2026-08-11 |

## 模板

```markdown
# ADR-NNNN：标题

- 状态：Proposed / Accepted / Deprecated / Superseded
- 日期：YYYY-MM-DD
- 关联：ADR-XXXX（可选）

## 背景（Context）

...

## 决策（Decision）

...

## 结果（Consequences）

...
```