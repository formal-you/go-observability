# 性能基线（2026-08-11）

## 环境

- 提交：`829e910`
- Go：`go1.26.5 windows/amd64`
- OS：Windows 11 Enterprise `10.0.26200`
- CPU：Intel Core i7-14700K，20 cores / 28 logical processors
- 运行：`GOMAXPROCS=1`、`GOGC=30`、`-benchtime=500ms`、`-count=3`

```powershell
go test ./log ./writer/file ./writer/otlp -run '^$' -bench 'Benchmark(Logger|Writer)' -benchmem -benchtime=500ms -count=3
```

## 结果

| Benchmark | 三次 ns/op | B/op | allocs/op | 范围 |
| --- | --- | --- | --- | --- |
| Logger Emit | 888.4 / 894.7 / 895.2 | 1600 | 3 | BusinessEvent 归一化，无 Masker/Sampler |
| Logger Emit + Mask + Sample | 21026 / 24330 / 28229 | 3112 | 7 | 默认敏感键递归检查 + Ratio=1 |
| 同步 file Writer | 7983 / 7960 / 7879 | 1728 | 42 | 7 个属性 + service.name，真实临时文件写入 |
| OTLP enqueue | 2080 / 2035 / 2035 | 919-920 | 3 | 5 个属性，SDK BatchProcessor 有界队列，不含网络导出 |

## 解读边界

本报告只建立回归比较基线，不是生产 SLO。真实门禁必须由部署方给出目标 QPS、并发数、典型/最大事件大小、CPU/内存预算和允许丢弃策略后再设定。同步 file Writer 的结果包含本机文件系统行为；OTLP 数字只测 enqueue，不代表 Collector 或后端吞吐。
