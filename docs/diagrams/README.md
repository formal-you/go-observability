# 图表导航（insight-diagram 输出）

本目录由 [insight-diagram](https://github.com/smallnest/goal-workflow/tree/master/skills/insight-diagram) skill 生成：
分析本仓库（go-observability）后输出 10 张 UML / 架构图，均为独立 HTML+SVG（light Claude 风格），双击即可在浏览器查看。

| 文件 | 类型 | 内容 |
| --- | --- | --- |
| [index.html](index.html) | 导航页 | 全部图表入口 |
| [architecture.html](architecture.html) | 架构图 | 分层架构 + 依赖方向 + 双投影 |
| [class.html](class.html) | 类图 | log/ 六类事件 + 接口、errs、logwriter、telemetry 核心类型 |
| [component.html](component.html) | 组件图 | 包级组件与依赖 |
| [package.html](package.html) | 包图 | 按层分组与模块依赖 |
| [deployment.html](deployment.html) | 部署图 | 应用进程 → Collector → LGTM；本地 JSONL |
| [usecase.html](usecase.html) | 用例图 | 三类参与者 × 12 项能力 |
| [sequence.html](sequence.html) | 序列图 | Logger.Emit 治理管线调用链 |
| [flowchart.html](flowchart.html) | 流程图 | 错误收口主流程与分支 |
| [activity.html](activity.html) | 活动图 | telemetry.Runtime 装配流程 |
| [swimlane.html](swimlane.html) | 泳道图 | 一次 HTTP 请求跨层全链路 |

## 校验

所有 SVG 均已通过几何校验（箭头落点 / 框重叠 / 框间距）：

```powershell
python "$env:USERPROFILE\.codex\skills\insight-diagram\scripts\review_svg.py" docs/diagrams/*.html --min-gap 8
```

## 重新生成

skill 安装于 `C:\Users\formal\.codex\skills\insight-diagram`（含 `examples/` 与 `scripts/`）。
新增/修改图表时：先读对应 `examples/<type>.html` 提取布局与样式，再按同风格生成 HTML+SVG，
最后运行上方校验脚本，`ERROR` 必须清零后再提交。
