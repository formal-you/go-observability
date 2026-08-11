package main

import (
	"os"
	"path/filepath"
	"testing"

	log "github.com/formal-you/go-observability/log"
)

func TestLoadBlackboxConfigTemplate(t *testing.T) {
	cfg, err := loadBlackboxConfig("config.example.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Service.Name != blackboxServiceName || cfg.Logs.OutputPath == "" {
		t.Fatalf("配置模板未加载基本字段: %+v", cfg)
	}
	if !cfg.Logs.Rotation.Enabled || cfg.Logs.Rotation.MaxSizeMB <= 0 {
		t.Fatalf("配置模板未启用有效轮转: %+v", cfg.Logs.Rotation)
	}
	level, err := configuredLevel(cfg.Logs.Level)
	if err != nil || level != log.LevelInfo {
		t.Fatalf("配置级别=%q/%v, want INFO", level, err)
	}
}

func TestLoadBlackboxConfigRejectsUnknownField(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	data := []byte("service:\n  name: svc\n  unknown: true\nlogs:\n  level: INFO\n  output_path: events.jsonl\notlp: {}\n")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := loadBlackboxConfig(path); err == nil {
		t.Fatal("未知配置字段应报错，防止拼写错误静默失效")
	}
}

func TestValidateBlackboxConfig(t *testing.T) {
	valid := defaultBlackboxConfig()
	tests := []struct {
		name   string
		mutate func(*blackboxConfig)
	}{
		{"missing service", func(cfg *blackboxConfig) { cfg.Service.Name = "" }},
		{"missing path", func(cfg *blackboxConfig) { cfg.Logs.OutputPath = "" }},
		{"invalid level", func(cfg *blackboxConfig) { cfg.Logs.Level = "NOTICE" }},
		{"invalid rotation", func(cfg *blackboxConfig) { cfg.Logs.Rotation.MaxSizeMB = 0 }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := valid
			tt.mutate(&cfg)
			if err := validateBlackboxConfig(cfg); err == nil {
				t.Fatalf("配置 %+v 应报错", cfg)
			}
		})
	}
}
