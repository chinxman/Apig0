package config

import (
	"fmt"
	"log"
	"os"
	"strings"
)

// VaultInterface is the universal contract for any secret backend.
// Implement this to add support for a new vault provider.
type VaultInterface interface {
	// LoadSecret fetches a secret. secretPath is the logical group (e.g. "totp"),
	// key is the identifier within that group (e.g. a username).
	LoadSecret(secretPath string, key string) (string, error)

	// Health returns nil if the backend is reachable and authenticated.
	Health() error

	// String returns the provider name for logging.
	String() string
}

// VaultConfig holds the common configuration shared by all providers.
// Provider-specific settings are read from their own env vars.
type VaultConfig struct {
	Type       string // env, hashicorp, aws, gcp, azure, cyberark, 1password, http, exec
	SecretPath string // logical secret group (default: "totp")
	SecretKey  string // key inside the secret data (default: "secret")
}

// LoadVaultConfig reads common vault settings from the environment.
func LoadVaultConfig() *VaultConfig {
	cfg := &VaultConfig{
		Type:       os.Getenv("VAULT_TYPE"),
		SecretPath: os.Getenv("VAULT_SECRET_PATH"),
		SecretKey:  os.Getenv("VAULT_SECRET_KEY"),
	}
	if cfg.Type == "" {
		cfg.Type = "env"
	}
	if cfg.SecretPath == "" {
		cfg.SecretPath = "totp"
	}
	if cfg.SecretKey == "" {
		cfg.SecretKey = "secret"
	}
	return cfg
}

// CreateVault builds the vault client for the configured provider.
func CreateVault(cfg *VaultConfig) (VaultInterface, error) {
	switch strings.ToLower(cfg.Type) {
	case "env":
		return NewEnvVault(), nil
	case "hashicorp", "vault":
		return NewHashicorpVault(cfg)
	case "aws":
		return NewAWSVault(cfg)
	case "gcp":
		return NewGCPVault(cfg)
	case "azure":
		return NewAzureVault(cfg)
	case "cyberark":
		return NewCyberArkVault(cfg)
	case "1password", "op":
		return New1PasswordVault(cfg)
	case "http":
		return NewHTTPVault(cfg)
	case "exec":
		return NewExecVault(cfg)
	default:
		return nil, fmt.Errorf("unsupported vault type: %q — supported: env, hashicorp, aws, gcp, azure, cyberark, 1password, http, exec", cfg.Type)
	}
}

// ---------------------------------------------------------------------------
// EnvVault — secrets from environment variables (no server needed)
// ---------------------------------------------------------------------------

type EnvVault struct{}

func NewEnvVault() *EnvVault { return &EnvVault{} }

func (v *EnvVault) LoadSecret(secretPath string, key string) (string, error) {
	// Try per-user: APIG0_TOTP_SECRET_DEVIN
	envName := "APIG0_TOTP_SECRET_" + strings.ToUpper(strings.ReplaceAll(key, "-", "_"))
	if val := os.Getenv(envName); val != "" {
		return val, nil
	}
	// Try generic: APIG0_TOTP_SECRET
	if val := os.Getenv("APIG0_TOTP_SECRET"); val != "" {
		return val, nil
	}
	return "", fmt.Errorf("set %s or APIG0_TOTP_SECRET", envName)
}

func (v *EnvVault) Health() error  { return nil }
func (v *EnvVault) String() string { return "env" }

// ---------------------------------------------------------------------------
// Shared state and helpers
// ---------------------------------------------------------------------------

// activeVault is the initialized vault client for the process lifetime.
var activeVault VaultInterface

// LoadVaultSecrets initializes the vault client and pre-loads secrets for
// all users listed in APIG0_USERS (comma-separated, default: "devin").
func LoadVaultSecrets() {
	cfg := LoadVaultConfig()

	vault, err := CreateVault(cfg)
	if err != nil {
		log.Printf("[vault] %s init failed: %v — falling back to env", cfg.Type, err)
		vault = NewEnvVault()
	} else {
		if err := vault.Health(); err != nil {
			log.Printf("[vault] %s health check failed: %v", vault, err)
		} else {
			log.Printf("[vault] connected to %s backend", vault)
		}
	}
	activeVault = vault

	users := "devin"
	if u := os.Getenv("APIG0_USERS"); u != "" {
		users = u
	}
	for _, user := range strings.Split(users, ",") {
		user = strings.TrimSpace(user)
		if user == "" {
			continue
		}
		secret, err := vault.LoadSecret(cfg.SecretPath, user)
		if err != nil {
			log.Printf("[vault] no secret for %q: %v", user, err)
			continue
		}
		UserSecrets[user] = secret
		log.Printf("[vault] loaded secret for %q", user)
	}
}

// LoadUserSecret fetches a single user's secret on demand.
// Checks in-memory cache first, then asks the active vault.
func LoadUserSecret(user string) string {
	if val, ok := UserSecrets[user]; ok {
		return val
	}
	if activeVault == nil {
		return ""
	}
	cfg := LoadVaultConfig()
	secret, err := activeVault.LoadSecret(cfg.SecretPath, user)
	if err != nil {
		return ""
	}
	UserSecrets[user] = secret
	return secret
}

