package otlpendpoint

import "testing"

func TestParse(t *testing.T) {
	tests := []struct {
		name     string
		endpoint string
		want     string
	}{
		{name: "hostname", endpoint: "collector:4317", want: "http://collector:4317"},
		{name: "IPv4", endpoint: "127.0.0.1:4317", want: "http://127.0.0.1:4317"},
		{name: "IPv6", endpoint: "[::1]:4317", want: "http://[::1]:4317"},
		{name: "HTTP URL", endpoint: "http://collector:4317", want: "http://collector:4317"},
		{name: "HTTPS URL", endpoint: "https://collector.example.com:4317/", want: "https://collector.example.com:4317/"},
		{name: "HTTPS default port", endpoint: "https://collector.example.com", want: "https://collector.example.com"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse(tt.endpoint)
			if err != nil {
				t.Fatalf("Parse(%q) error = %v", tt.endpoint, err)
			}
			if got != tt.want {
				t.Fatalf("Parse(%q) = %q, want %q", tt.endpoint, got, tt.want)
			}
		})
	}
}

func TestParseRejectsInvalidEndpoint(t *testing.T) {
	invalid := []string{
		"",
		"collector",
		"collector:",
		":4317",
		"collector:not-a-port",
		"collector:0",
		"collector:70000",
		"collector/path:4317",
		"collector@evil:4317",
		"collector?tenant=mall:4317",
		"grpc://collector:4317",
		"http://",
		"https://collector:0",
		"https://collector:70000",
		"https://user@collector:4317",
		"https://collector:4317/v1/logs",
		"https://collector:4317?tenant=mall",
		"https://collector:4317#fragment",
	}
	for _, endpoint := range invalid {
		t.Run(endpoint, func(t *testing.T) {
			if _, err := Parse(endpoint); err == nil {
				t.Fatalf("Parse(%q) error = nil, want non-nil", endpoint)
			}
		})
	}
}
