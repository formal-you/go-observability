// Package otlpendpoint 提供 OTLP gRPC endpoint 的统一校验与规范化。
package otlpendpoint

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
)

// Parse 校验 OTLP gRPC endpoint 并返回 URL 形式。
// 裸 host:port 明确按明文连接规范为 http://host:port；http(s) URL 保留其 scheme。
func Parse(endpoint string) (string, error) {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return "", fmt.Errorf("OTLP endpoint is empty")
	}

	if !strings.Contains(endpoint, "://") {
		if err := validateHostPort(endpoint); err != nil {
			return "", err
		}
		endpoint = "http://" + endpoint
	}

	u, err := url.ParseRequestURI(endpoint)
	if err != nil {
		return "", fmt.Errorf("invalid OTLP endpoint URL %q: %w", endpoint, err)
	}
	u.Scheme = strings.ToLower(u.Scheme)
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("OTLP endpoint URL scheme must be http or https: %q", endpoint)
	}
	if u.Hostname() == "" {
		return "", fmt.Errorf("OTLP endpoint URL has no host: %q", endpoint)
	}
	if port := u.Port(); port != "" {
		if err := validatePort(port, endpoint); err != nil {
			return "", err
		}
	}
	if u.User != nil || (u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.Fragment != "" {
		return "", fmt.Errorf("OTLP endpoint URL must not contain user info, path, query, or fragment: %q", endpoint)
	}
	return u.String(), nil
}

func validateHostPort(endpoint string) error {
	host, port, err := net.SplitHostPort(endpoint)
	if err != nil {
		return fmt.Errorf("OTLP endpoint must be host:port or an http(s) URL: %q: %w", endpoint, err)
	}
	if host == "" {
		return fmt.Errorf("OTLP endpoint host is empty: %q", endpoint)
	}
	if port == "" {
		return fmt.Errorf("OTLP endpoint port is empty: %q", endpoint)
	}
	return validatePort(port, endpoint)
}

func validatePort(port, endpoint string) error {
	n, err := strconv.Atoi(port)
	if err != nil || n < 1 || n > 65535 {
		return fmt.Errorf("OTLP endpoint has invalid port: %q", endpoint)
	}
	return nil
}
