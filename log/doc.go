// Package log 定义面向 Go 服务的类型化语义日志核心抽象。
//
// 依赖方向：本包只依赖标准库（log/slog、net、time），不依赖任何 OTel 实现。
// 属性键直接使用 OTel semconv 名称（见 keys.go），Resource、severity、trace 关联
// 由外层装配（Writer 适配层 + OTel SDK）负责；业务调用方只构造 Payload 并交给
// Logger 写出；采样与脱敏通过 Sampler、Masker 显式注入，默认均不启用。
package log
