package config

import (
	"os"
	"strings"
)

// UserSecrets contains TOTP secrets for users
// Use APIG0_TOTP_SECRET env var to override the default secret
var UserSecrets = map[string]string{
	"devin": func() string {
		if secret := os.Getenv("APIG0_TOTP_SECRET"); secret != "" {
			return secret
		}
		return "WECNPPUPNEXZDYNNDBJYIPHJWUXXCQ5P" // default for testing
	}(),
}

// GatewayConfig holds configurable gateway settings
type GatewayConfig struct {
	TrustedProxies   []string
	CORSAllowedOrigins []string
	Port             string
	ShowQR           bool
}

// DefaultConfig returns a config with defaults, overridden by env vars
func DefaultConfig() *GatewayConfig {
	config := &GatewayConfig{
		TrustedProxies:     []string{"127.0.0.1", "192.168.12.0/24"},
		CORSAllowedOrigins: []string{"*"},
		Port:               "8080",
		ShowQR:           os.Getenv("APIG0_SHOW_QR") == "true",
	}

	// Allow overriding via env vars
	if proxies := os.Getenv("APIG0_TRUSTED_PROXIES"); proxies != "" {
		config.TrustedProxies = splitCSV(proxies)
	}
	if origins := os.Getenv("APIG0_CORS_ORIGINS"); origins != "" {
		config.CORSAllowedOrigins = splitCSV(origins)
	}
	if port := os.Getenv("APIG0_PORT"); port != "" {
		config.Port = port
	}

	return config
}

func splitCSV(csv string) []string {
	var parts []string
	for _, p := range split(csv, ',') {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			parts = append(parts, trimmed)
		}
	}
	return parts
}

func split(s string, sep byte) []string {
	var result []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == sep {
			result = append(result, s[start:i])
			start = i + 1
		}
	}
	result = append(result, s[start:])
	return result
}
