package config

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// VaultInterface is the universal contract for any secret backend.
// Implement this to add support for a new vault provider.
type VaultInterface interface {
	// LoadSecret fetches a secret value by path and key.
	LoadSecret(secretPath string, key string) (string, error)

	// StoreSecret writes a secret value. Returns ErrReadOnly if not supported.
	StoreSecret(secretPath string, key string, value string) error

	// DeleteSecret removes a secret. Returns ErrReadOnly if not supported.
	DeleteSecret(secretPath string, key string) error

	// ListKeys returns all keys under a path. Returns ErrReadOnly if not supported.
	ListKeys(secretPath string) ([]string, error)

	// Health returns nil if the backend is reachable and authenticated.
	Health() error

	// String returns the provider name for logging.
	String() string
}

// ErrReadOnly is returned by vault backends that do not support write operations.
var ErrReadOnly = fmt.Errorf("vault backend is read-only")

// VaultConfig holds the common configuration shared by all providers.
// Provider-specific settings are read from their own env vars.
type VaultConfig struct {
	Type       string // env, hashicorp, aws, gcp, azure, cyberark, 1password, http, exec
	SecretPath string // logical secret group (default: "totp")
	SecretKey  string // key inside the secret data (default: "secret")
	FilePath   string // local file for VAULT_TYPE=file
}

// LoadVaultConfig reads common vault settings from the environment.
func LoadVaultConfig() *VaultConfig {
	cfg := &VaultConfig{
		Type:       os.Getenv("VAULT_TYPE"),
		SecretPath: os.Getenv("VAULT_SECRET_PATH"),
		SecretKey:  os.Getenv("VAULT_SECRET_KEY"),
		FilePath:   os.Getenv("VAULT_FILE_PATH"),
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
	if cfg.FilePath == "" {
		cfg.FilePath = "totp-secrets.json"
	}
	return cfg
}

// CreateVault builds the vault client for the configured provider.
func CreateVault(cfg *VaultConfig) (VaultInterface, error) {
	switch strings.ToLower(cfg.Type) {
	case "env":
		return NewEnvVault(), nil
	case "file":
		return NewFileVault(cfg), nil
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
		return nil, fmt.Errorf("unsupported vault type: %q — supported: env, file, hashicorp, aws, gcp, azure, cyberark, 1password, http, exec", cfg.Type)
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

func (v *EnvVault) StoreSecret(secretPath, key, value string) error {
	UserSecrets[key] = value
	return nil
}

func (v *EnvVault) DeleteSecret(secretPath, key string) error {
	delete(UserSecrets, key)
	return nil
}

func (v *EnvVault) ListKeys(secretPath string) ([]string, error) {
	keys := make([]string, 0, len(UserSecrets))
	for k := range UserSecrets {
		keys = append(keys, k)
	}
	return keys, nil
}

func (v *EnvVault) Health() error  { return nil }
func (v *EnvVault) String() string { return "env" }

// ---------------------------------------------------------------------------
// FileVault — secrets stored in a local JSON file for persistent local setups
// ---------------------------------------------------------------------------

type FileVault struct {
	mu       sync.Mutex
	filePath string
}

type fileVaultData struct {
	Secrets map[string]string `json:"secrets"`
}

func NewFileVault(cfg *VaultConfig) *FileVault {
	return &FileVault{filePath: cfg.FilePath}
}

func (v *FileVault) LoadSecret(secretPath string, key string) (string, error) {
	data, err := v.readAll()
	if err != nil {
		return "", err
	}
	secret, ok := data.Secrets[key]
	if !ok || secret == "" {
		return "", fmt.Errorf("not found: %s/%s", secretPath, key)
	}
	return secret, nil
}

func (v *FileVault) StoreSecret(secretPath, key, value string) error {
	data, err := v.readAll()
	if err != nil {
		return err
	}
	data.Secrets[key] = value
	return v.writeAll(data)
}

func (v *FileVault) DeleteSecret(secretPath, key string) error {
	data, err := v.readAll()
	if err != nil {
		return err
	}
	delete(data.Secrets, key)
	return v.writeAll(data)
}

func (v *FileVault) ListKeys(secretPath string) ([]string, error) {
	data, err := v.readAll()
	if err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(data.Secrets))
	for key := range data.Secrets {
		keys = append(keys, key)
	}
	return keys, nil
}

func (v *FileVault) Health() error { return nil }
func (v *FileVault) String() string {
	return "file"
}

func (v *FileVault) Path() string {
	return v.filePath
}

func (v *FileVault) readAll() (*fileVaultData, error) {
	v.mu.Lock()
	defer v.mu.Unlock()

	data := &fileVaultData{Secrets: map[string]string{}}
	raw, err := os.ReadFile(v.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return data, nil
		}
		return nil, err
	}
	if len(raw) == 0 {
		return data, nil
	}
	if err := json.Unmarshal(raw, data); err != nil {
		return nil, err
	}
	if data.Secrets == nil {
		data.Secrets = map[string]string{}
	}
	return data, nil
}

func (v *FileVault) writeAll(data *fileVaultData) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	dir := filepath.Dir(v.filePath)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return err
		}
	}

	raw, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(v.filePath, raw, 0600)
}

// ---------------------------------------------------------------------------
// Shared state and helpers
// ---------------------------------------------------------------------------

// activeVault is the initialized vault client for the process lifetime.
var activeVault VaultInterface

// ActiveVaultName returns the active secret backend name.
func ActiveVaultName() string {
	if activeVault == nil {
		return ""
	}
	return activeVault.String()
}

// ActiveVaultFilePath returns the local secret file path when VAULT_TYPE=file.
func ActiveVaultFilePath() string {
	if v, ok := activeVault.(*FileVault); ok {
		return v.Path()
	}
	return ""
}

// LoadVaultSecrets initializes the vault client and pre-loads secrets for
// all users listed in APIG0_USERS.
func LoadVaultSecrets() {
	cfg := LoadVaultConfig()
	UserSecrets = map[string]string{}

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

	for _, user := range configuredUsers() {
		secret, err := vault.LoadSecret(cfg.SecretPath, user)
		if err != nil {
			log.Printf("[vault] no secret for %q: %v", user, err)
			continue
		}
		UserSecrets[user] = secret
		log.Printf("[vault] loaded secret for %q", user)
	}
}

// StoreUserSecret stores a TOTP secret for a user in the active vault and in memory.
func StoreUserSecret(username, secret string) error {
	cfg := LoadVaultConfig()
	if activeVault != nil {
		if err := activeVault.StoreSecret(cfg.SecretPath, username, secret); err != nil {
			return err
		}
	}
	UserSecrets[username] = secret
	return nil
}

// DeleteUserSecret removes a user's TOTP secret from the active vault and memory.
func DeleteUserSecret(username string) {
	cfg := LoadVaultConfig()
	if activeVault != nil {
		activeVault.DeleteSecret(cfg.SecretPath, username)
	}
	delete(UserSecrets, username)
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
