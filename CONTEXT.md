# go-observability 领域上下文

go-observability 是 Go 语义化日志与可观测装配库：定义事件如何分类、命名、治理（采样/脱敏）与输出，并统一 Trace / Metric / Log 三信号装配的术语。本文件是术语表（ubiquitous language），不是规格或实现说明；术语冲突时以本文件为准。

## Language

### 事件模型

**事件（Event）**:
一次语义化日志记录，由粗分类（EventType）、细名（EventName）与扁平属性组成，经 Logger 治理后由 Writer 输出。
_Avoid_: 日志行、log message（指结构化事件时）

**EventType**:
事件的粗分类，固定六类：access / business / error / security / audit / probe；随事件写为 msg。
_Avoid_: 日志级别（Level 是另一维度）、EventName

**EventName**:
事件的细名，三段式「类别.模块.操作」（如 access.http.request、business.order.paid），写入 event.name，三段式必须经 Validate。
_Avoid_: 随意字符串、message、EventType

**Level**:
语义化级别 DEBUG / INFO / WARN / ERROR，映射 slog.Level 与 OTel SeverityNumber。
_Avoid_: HTTP 状态码

**Result**:
跨事件类型的业务结果：success / failed / error / blocked / denied / unknown。
_Avoid_: HTTP 状态码（状态码是访问事件的传输字段，不等于 Result）

**高价值结果（High-value Result）**:
failed / error / blocked / denied 四类，采样器强制保留。
_Avoid_: 错误（仅指 error 一类）、异常

### 数据治理

**采样（Sampling）**:
按策略选择哪些事件保留、哪些丢弃的选择性减量；采样率（如 0.1）表示保留比例。
_Avoid_: 采集频率、导出频率、批处理

**批量导出间隔（Batch Export Interval）**:
SDK 侧多久打包导出一次（trace 5s / metric 15s / log 1s）与 Collector 侧 batch.timeout（凑批等待）；只影响时延与批大小，不丢数据。
_Avoid_: 采样、刷新频率、采集频率

**采集频率（避免使用）**:
仓库不采用此模糊词；精确说法是「批量导出间隔」（多久发一次）或「采样率」（留哪些）。
_Avoid_: 用它描述导出或采样

**头部采样（Head Sampling）**:
SDK 在导出前按 trace id 概率采样（TraceSampleRatio）；被丢弃的 trace 永远到不了 Collector，尾部采样无法恢复。
_Avoid_: 前端采样

**尾部采样（Tail Sampling）**:
Collector 侧等 trace 完整到达后按错误 / 概率 / 属性策略决定保留。
_Avoid_: 后端采样（模糊）

**Sampler**:
采样判定器，返回保留或丢弃；ResultKeepSampler 按 Result 判定，EventKeepSampler 按 event.name 前缀 + Fallback 判定。
_Avoid_: 采样频率、过滤器

**Masker（脱敏）**:
写出前按键名递归替换敏感值（覆盖 group / map / slice / LogValuer）。
_Avoid_: 加密、匿名化

**Writer**:
日志输出后端（file / stdout / otlp），位于治理管线末端，看到的是已完成采样与脱敏的扁平事件。
_Avoid_: Exporter（不同层）

### 遥测与装配

**层（SDK 侧 / Collector 侧）**:
频率决策分两层：应用 SDK 决定采样率与批量导出间隔；Collector 只做接收端 batch 凑批与可选尾部采样，管不到 SDK 何时导出。
_Avoid_: 用 Collector 配置描述 SDK 采样率或导出间隔

**Exporter**:
telemetry 侧的 OTel 导出器（otlptrace / otlpmetric / otlplog），与根包 Writer 不同层。
_Avoid_: Writer

**Provider**:
trace / metric / log 三信号 Provider 的装配与全局安装（otel / global 包）。
_Avoid_: Exporter

**Resource**:
服务身份与低基数标签（service.name、service.version、deployment.environment、region、instance）。
_Avoid_: 属性（attribute 是事件字段维度）

**Pipeline（管线）**:
Collector 中 receiver → processor → exporter 的链路；本仓库三信号各一条（traces / metrics / logs）。
_Avoid_: 数据流图、Writer

### 输出与归一化

**双投影（Dual Projection）**:
同一事件面向两个正确形状：file/stdout 扁平运营列（timestamp / level / trace_id / event.name…），OTLP 映射 LogRecord 顶层字段（Severity / EventName / Timestamp / span context）。
_Avoid_: 一份数据两种格式（会误解为重复数据）

**归一化（Normalization）**:
metadata + payload 合并为扁平 attrs，过滤保留键与零值省略。
_Avoid_: 序列化（序列化在 Writer 层）

**心跳 / 健康检查（Heartbeat / Health Check）**:
确定性高频噪音（如 /healthz），用 SkipPaths 或接入层短路排除，不进入概率采样。
_Avoid_: 访问事件（心跳不是业务访问）
