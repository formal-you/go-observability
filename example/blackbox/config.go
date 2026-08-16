package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	log "github.com/formal-you/go-observability/log"
	"github.com/formal-you/go-observability/logwriter/file"
	"gopkg.in/yaml.v3"
)

type blackboxConfig struct {
	Service serviceConfig `yaml:"service"`
	Logs    logsConfig    `yaml:"logs"`
	OTLP    otlpConfig    `yaml:"otlp"`
}

type serviceConfig struct {
	Name        string `yaml:"name"`
	Version     string `yaml:"version"`
	InstanceID  string `yaml:"instance_id"`
	Environment string `yaml:"environment"`
}

type logsConfig struct {
	Level            string         `yaml:"level"`
	OutputPath       string         `yaml:"output_path"`
	OverwriteOnStart bool           `yaml:"overwrite_on_start"`
	Rotation         rotationConfig `yaml:"rotation"`
}

type rotationConfig struct {
	Enabled      bool `yaml:"enabled"`
	MaxSizeMB    int  `yaml:"max_size_mb"`
	MaxBackups   int  `yaml:"max_backups"`
	MaxAgeDays   int  `yaml:"max_age_days"`
	Compress     bool `yaml:"compress"`
	UseLocalTime bool `yaml:"use_local_time"`
}

type otlpConfig struct {
	Endpoint string `yaml:"endpoint"`
}

func defaultBlackboxConfig() blackboxConfig {
	return blackboxConfig{
		Service: serviceConfig{
			Name:        blackboxServiceName,
			Version:     "dev",
			InstanceID:  "blackbox-local",
			Environment: "local",
		},
		Logs: logsConfig{
			Level:            "INFO",
			OutputPath:       "example/blackbox/sample.jsonl",
			OverwriteOnStart: true,
			Rotation: rotationConfig{
				Enabled:      true,
				MaxSizeMB:    10,
				MaxBackups:   5,
				MaxAgeDays:   7,
				Compress:     true,
				UseLocalTime: true,
			},
		},
		OTLP: otlpConfig{Endpoint: "127.0.0.1:4317"},
	}
}

func loadBlackboxConfig(path string) (blackboxConfig, error) {
	f, err := os.Open(path)
	if err != nil {
		return blackboxConfig{}, fmt.Errorf("打开配置文件: %w", err)
	}
	defer f.Close()
	var cfg blackboxConfig
	decoder := yaml.NewDecoder(f)
	decoder.KnownFields(true)
	if err := decoder.Decode(&cfg); err != nil {
		return blackboxConfig{}, fmt.Errorf("解析配置文件: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		if err == nil {
			return blackboxConfig{}, errors.New("解析配置文件: 只能包含一份 YAML 文档")
		}
		return blackboxConfig{}, fmt.Errorf("解析配置文件尾部: %w", err)
	}
	if err := validateBlackboxConfig(cfg); err != nil {
		return blackboxConfig{}, err
	}
	return cfg, nil
}

func validateBlackboxConfig(cfg blackboxConfig) error {
	if strings.TrimSpace(cfg.Service.Name) == "" {
		return errors.New("配置 service.name 必填")
	}
	if strings.TrimSpace(cfg.Logs.OutputPath) == "" {
		return errors.New("配置 logs.output_path 必填")
	}
	if _, err := configuredLevel(cfg.Logs.Level); err != nil {
		return err
	}
	if cfg.Logs.Rotation.Enabled {
		rotation := cfg.Logs.Rotation.fileConfig()
		if rotation.MaxSizeMB <= 0 {
			return errors.New("配置 logs.rotation.max_size_mb 必须大于 0")
		}
		if rotation.MaxBackups < 0 || rotation.MaxAgeDays < 0 {
			return errors.New("配置 logs.rotation 的备份数量和保留天数不能为负数")
		}
	}
	return nil
}

func configuredLevel(value string) (log.Level, error) {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "DEBUG":
		return log.LevelDebug, nil
	case "INFO":
		return log.LevelInfo, nil
	case "WARN":
		return log.LevelWarn, nil
	case "ERROR":
		return log.LevelError, nil
	default:
		return "", fmt.Errorf("配置 logs.level=%q 无效，可选 DEBUG/INFO/WARN/ERROR", value)
	}
}

func (cfg rotationConfig) fileConfig() file.RotationConfig {
	return file.RotationConfig{
		MaxSizeMB:  cfg.MaxSizeMB,
		MaxBackups: cfg.MaxBackups,
		MaxAgeDays: cfg.MaxAgeDays,
		Compress:   cfg.Compress,
		LocalTime:  cfg.UseLocalTime,
	}
}
