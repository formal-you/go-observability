package telemetry

import (
	"context"
	"errors"
	"os"
	"strings"

	"github.com/formal-you/go-observability/log"
	"github.com/formal-you/go-observability/writer/file"
	"github.com/formal-you/go-observability/writer/otlp"
)

// Setup 使用旧版 endpoint 推断规则创建 Runtime，并立即安装为 global Provider。
// Endpoint 非空时选择 OTLP，否则选择 file；该隐式规则仅为兼容旧接入保留。
// Deprecated: 使用 NewRuntime 和 Runtime.InstallGlobal。
func Setup(ctx context.Context, cfg Config) (*Runtime, error) {
	if cfg.Enabled {
		if strings.TrimSpace(cfg.Endpoint) != "" {
			cfg.LogOutput = LogOutputOTLP
		} else {
			cfg.LogOutput = LogOutputFile
		}
	}
	r, err := NewRuntime(ctx, cfg)
	if err != nil {
		return nil, err
	}
	if !cfg.Enabled {
		r.logOutput = LogOutputFile
	}
	r.InstallGlobal()
	return r, nil
}

// SetupFile 创建 file-only Runtime，并立即安装为 global Provider。
// Deprecated: 使用 NewFileRuntime 和 Runtime.InstallGlobal。
func SetupFile(cfg Config) (*Runtime, error) {
	r, err := NewFileRuntime(cfg)
	if err != nil {
		return nil, err
	}
	r.InstallGlobal()
	return r, nil
}

// SetupFromEnvironment 读取旧版环境变量映射后调用 Setup。
// Deprecated: 使用 NewRuntime 和显式 Config 字段。
func SetupFromEnvironment(ctx context.Context, cfg Config) (*Runtime, error) {
	cfg.Enabled = EnabledFromEnvironment()
	if strings.TrimSpace(cfg.Endpoint) == "" {
		cfg.Endpoint = strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"))
	}
	return Setup(ctx, cfg)
}

// NewLogWriter 把旧版路径与 file options 映射到当前 Writer 实现。
//
// 为保持源代码和具体类型兼容，本方法仍返回 log.Writer，而不是 ManagedWriter；新代码应
// 使用 NewWriter 获得统一且幂等的 Close。OTLP 模式复用 Runtime 的 LoggerProvider，
// Writer 不拥有也不会关闭该 Provider。
// Deprecated: 使用 Runtime.NewWriter。
func (r *Runtime) NewLogWriter(ctx context.Context, jsonlPath string, fileOpts ...file.Option) (log.Writer, error) {
	if r != nil && r.logOutput == LogOutputOTLP {
		if r.loggerProvider == nil {
			return nil, errors.New("telemetry: otlp output requires logger provider")
		}
		return otlp.New(ctx, otlp.WithLoggerProvider(r.loggerProvider), otlp.WithResource(r.resource))
	}
	if r == nil {
		return nil, errors.New("telemetry: nil runtime")
	}
	opts := append([]file.Option(nil), fileOpts...)
	opts = append(opts, file.WithResourceMetadata(r.fileMetadata))
	return file.New(jsonlPath, opts...)
}
