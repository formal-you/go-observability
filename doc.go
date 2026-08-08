// Package log 定义商城可观测性日志的核心抽象（方案2：直接符合 OTel 语义约定）。
//
// 依赖方向：本包只依赖标准库（log/slog、net、time），不依赖任何 OTel 实现。
// 属性键直接使用 OTel semconv 名称（见 keys.go），Resource、severity、trace 关联
// 由外层装配（Writer 适配层 + OTel SDK）负责；业务调用方只构造 Payload 并交给
// Logger 写出，核心包不做转换、不做采样、不做脱敏（采样/脱敏由外层注入）。
package log
