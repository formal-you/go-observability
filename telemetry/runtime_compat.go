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

// Setup creates a Runtime using legacy endpoint-based output selection and
// installs it globally.
// Deprecated: use NewRuntime and Runtime.InstallGlobal.
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

// SetupFile creates a local file Runtime and installs it globally.
// Deprecated: use NewFileRuntime and Runtime.InstallGlobal.
func SetupFile(cfg Config) (*Runtime, error) {
	r, err := NewFileRuntime(cfg)
	if err != nil {
		return nil, err
	}
	r.InstallGlobal()
	return r, nil
}

// SetupFromEnvironment applies the legacy environment mapping before Setup.
// Deprecated: use NewRuntime and explicit Config fields.
func SetupFromEnvironment(ctx context.Context, cfg Config) (*Runtime, error) {
	cfg.Enabled = EnabledFromEnvironment()
	if strings.TrimSpace(cfg.Endpoint) == "" {
		cfg.Endpoint = strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"))
	}
	return Setup(ctx, cfg)
}

// NewLogWriter maps the legacy path and file options to NewWriter.
// Deprecated: use Runtime.NewWriter.
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
