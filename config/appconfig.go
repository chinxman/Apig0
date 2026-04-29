package config

import (
	"log"
	"os"

	"github.com/goccy/go-yaml"
)

type appConfig struct {
	Vault   vaultYAML   `yaml:"vault"`
	Gateway gatewayYAML `yaml:"gateway"`
}

type vaultYAML struct {
	Type       string `yaml:"type"`
	Address    string `yaml:"address"`
	Engine     string `yaml:"engine"`
	SecretPath string `yaml:"secret_path"`
	SecretKey  string `yaml:"secret_key"`
	FilePath   string `yaml:"file_path"`
}

type gatewayYAML struct {
	Users      string `yaml:"users"`
	Port       string `yaml:"port"`
	SessionTTL string `yaml:"session_ttl"`
	ShowQR     bool   `yaml:"show_qr"`
	TLS        string `yaml:"tls"`
}

// LoadAppConfig reads apig0.yaml from the working directory and sets any
// value as an env var default — only if that env var is not already set.
// Env vars always win, so existing test/dev exports are never overridden.
func LoadAppConfig() {
	data, err := os.ReadFile("apig0.yaml")
	if err != nil {
		if os.IsNotExist(err) {
			return // no file is fine, env vars take over
		}
		log.Printf("[config] warning: could not read apig0.yaml: %v", err)
		return
	}

	var cfg appConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		log.Printf("[config] warning: could not parse apig0.yaml: %v", err)
		return
	}

	setDefault("VAULT_TYPE", cfg.Vault.Type)
	setDefault("VAULT_ADDRESS", cfg.Vault.Address)
	setDefault("VAULT_ENGINE", cfg.Vault.Engine)
	setDefault("VAULT_SECRET_PATH", cfg.Vault.SecretPath)
	setDefault("VAULT_SECRET_KEY", cfg.Vault.SecretKey)
	setDefault("VAULT_FILE_PATH", cfg.Vault.FilePath)
	setDefault("APIG0_USERS", cfg.Gateway.Users)
	setDefault("APIG0_PORT", cfg.Gateway.Port)
	setDefault("APIG0_SESSION_TTL", cfg.Gateway.SessionTTL)
	setDefault("APIG0_TLS", cfg.Gateway.TLS)
	if cfg.Gateway.ShowQR {
		setDefault("APIG0_SHOW_QR", "true")
	}

	log.Printf("[config] apig0.yaml loaded")
}

// setDefault sets an env var only if it is not already defined.
func setDefault(key, value string) {
	if value == "" {
		return
	}
	if os.Getenv(key) == "" {
		os.Setenv(key, value)
	}
}
