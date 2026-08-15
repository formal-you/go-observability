# ADR-0003：核心 log 包从模块根目录迁移至 log/ 子目录

- 状态：Accepted
- 日期：2026-08-11
- 关联：ADR-0001 / ADR-0002（错误模型决策）

## 背景（Context）

核心日志包（package log）原本位于模块根目录，导入路径等于模块路径
`github.com/formal-you/go-observability`：包名（log）与目录名（根目录）不一致，
公共 API 与仓库根层混放，与 errs/、writer/、middleware/ 等子包结构不对等。
用户创建 log/ 目录，要求把日志项目代码收敛到该目录。

## 决策（Decision）

- 将根目录 20 个 .go 文件（doc.go、types.go、keys.go、log.go、payload.go、events.go、
  normalize.go、masker.go、metadata.go、sampler*.go、error_project*.go、multi_writer*.go
  及黑盒测试等）整体迁移至 log/ 子目录。
- 包导入路径由 `github.com/formal-you/go-observability` 改为
  `github.com/formal-you/go-observability/log`；模块路径（go.mod module）保持不变。
- 同步更新本模块内 21 处 import，以及工作区（go.work）内三个依赖方
  ai-gateway（5 处）、kratos-mall（5 处）、mall（12 处）的 import，保证整仓可编译。

## 结果（Consequences）

- 正面：包名 = 目录名（log/），结构对等清晰；模块根不再承载具体包，公共 API 归属明确。
- 代价：破坏性变更——所有依赖方需一次性迁移导入路径；对外示例与文档同步更新。
- 兼容性：模块路径与 go.work 不变，无 go.mod 改动；迁移与依赖更新在同一提交内完成。