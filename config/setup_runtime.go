package config

import (
	"crypto/rand"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
)

type SetupMode string

const (
	SetupModeTemporary  SetupMode = "temporary"
	SetupModePersistent SetupMode = "persistent"
)

type UserVaultSettings struct {
	Type       string `json:"type"`
	Address    string `json:"address,omitempty"`
	Engine     string `json:"engine,omitempty"`
	SecretPath string `json:"secret_path,omitempty"`
	SecretKey  string `json:"secret_key,omitempty"`
	FilePath   string `json:"file_path,omitempty"`
}

type SetupConfig struct {
	Mode              SetupMode           `json:"mode"`
	Port              string              `json:"port"`
	UsersPath         string              `json:"users_path,omitempty"`
	ServicesPath      string              `json:"services_path,omitempty"`
	RateLimitsPath    string              `json:"ratelimits_path,omitempty"`
	UserVault         UserVaultSettings   `json:"user_vault"`
	ServiceSecrets    ServiceSecretConfig `json:"service_secrets"`
	RequiresSetup     bool                `json:"-"`
	Persisted         bool                `json:"-"`
	MasterPasswordSet bool                `json:"-"`
}

var (
	setupMu         sync.RWMutex
	setupFilePath   = "apig0-setup.json"
	activeSetup     = defaultSetupConfig()
	setupConfigured bool
)

func defaultSetupConfig() SetupConfig {
	return SetupConfig{
		Mode:           SetupModeTemporary,
		Port:           "8080",
		UsersPath:      filepath.Join(os.TempDir(), "apig0-users-"+randomSuffix()+".json"),
		ServicesPath:   filepath.Join(os.TempDir(), "apig0-services-"+randomSuffix()+".json"),
		RateLimitsPath: filepath.Join(os.TempDir(), "apig0-ratelimits-"+randomSuffix()+".json"),
		UserVault: UserVaultSettings{
			Type:       "env",
			SecretPath: "totp",
			SecretKey:  "secret",
		},
		ServiceSecrets: ServiceSecretConfig{Mode: ServiceSecretMemory},
		RequiresSetup:  true,
	}
}

func LoadSetupBootstrap() SetupConfig {
	setupMu.Lock()
	defer setupMu.Unlock()

	activeSetup = defaultSetupConfig()
	setupConfigured = false

	raw, err := os.ReadFile(setupFilePath)
	if err != nil {
		applySetupEnvLocked(activeSetup)
		return activeSetup
	}

	var cfg SetupConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		applySetupEnvLocked(activeSetup)
		return activeSetup
	}
	cfg = normalizeSetupConfig(cfg)
	cfg.RequiresSetup = false
	cfg.Persisted = true
	activeSetup = cfg
	setupConfigured = true
	applySetupEnvLocked(activeSetup)
	return activeSetup
}

func CurrentSetupConfig() SetupConfig {
	setupMu.RLock()
	defer setupMu.RUnlock()
	return activeSetup
}

func SetupConfigured() bool {
	setupMu.RLock()
	defer setupMu.RUnlock()
	return setupConfigured
}

func SavePersistentSetup(cfg SetupConfig) error {
	setupMu.Lock()
	defer setupMu.Unlock()

	cfg = normalizeSetupConfig(cfg)
	raw, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(setupFilePath, raw, 0600); err != nil {
		return err
	}

	cfg.RequiresSetup = false
	cfg.Persisted = true
	activeSetup = cfg
	setupConfigured = true
	applySetupEnvLocked(activeSetup)
	return nil
}

func ActivateTemporarySetup(cfg SetupConfig) {
	setupMu.Lock()
	defer setupMu.Unlock()

	cfg = normalizeSetupConfig(cfg)
	cfg.RequiresSetup = false
	cfg.Persisted = false
	activeSetup = cfg
	setupConfigured = false
	applySetupEnvLocked(activeSetup)
}

func ResetSetupState() error {
	setupMu.Lock()
	defer setupMu.Unlock()

	paths := resetTargetsLocked()
	var firstErr error
	for _, path := range paths {
		if strings.TrimSpace(path) == "" {
			continue
		}
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) && firstErr == nil {
			firstErr = err
		}
	}

	activeSetup = defaultSetupConfig()
	setupConfigured = false
	applySetupEnvLocked(activeSetup)
	return firstErr
}

func normalizeSetupConfig(cfg SetupConfig) SetupConfig {
	if cfg.Mode == "" {
		cfg.Mode = SetupModeTemporary
	}
	if strings.TrimSpace(cfg.Port) == "" {
		cfg.Port = "8080"
	}
	if cfg.Mode == SetupModePersistent {
		if cfg.UsersPath == "" {
			cfg.UsersPath = "users.json"
		}
		if cfg.ServicesPath == "" {
			cfg.ServicesPath = "services.json"
		}
		if cfg.RateLimitsPath == "" {
			cfg.RateLimitsPath = "ratelimits.json"
		}
	} else {
		if cfg.UsersPath == "" {
			cfg.UsersPath = filepath.Join(os.TempDir(), "apig0-users-"+randomSuffix()+".json")
		}
		if cfg.ServicesPath == "" {
			cfg.ServicesPath = filepath.Join(os.TempDir(), "apig0-services-"+randomSuffix()+".json")
		}
		if cfg.RateLimitsPath == "" {
			cfg.RateLimitsPath = filepath.Join(os.TempDir(), "apig0-ratelimits-"+randomSuffix()+".json")
		}
	}
	if cfg.UserVault.Type == "" {
		cfg.UserVault.Type = "env"
	}
	if cfg.UserVault.SecretPath == "" {
		cfg.UserVault.SecretPath = "totp"
	}
	if cfg.UserVault.SecretKey == "" {
		cfg.UserVault.SecretKey = "secret"
	}
	if cfg.UserVault.Type == "file" && cfg.UserVault.FilePath == "" {
		cfg.UserVault.FilePath = "totp-secrets.json"
	}
	if cfg.ServiceSecrets.Mode == "" {
		if cfg.Mode == SetupModePersistent {
			cfg.ServiceSecrets.Mode = ServiceSecretFile
			cfg.ServiceSecrets.FilePath = "service-secrets.json"
		} else {
			cfg.ServiceSecrets.Mode = ServiceSecretMemory
		}
	}
	if cfg.ServiceSecrets.Mode != ServiceSecretMemory && cfg.ServiceSecrets.FilePath == "" {
		if cfg.ServiceSecrets.Mode == ServiceSecretEncryptedFile {
			cfg.ServiceSecrets.FilePath = "service-secrets.enc.json"
		} else {
			cfg.ServiceSecrets.FilePath = "service-secrets.json"
		}
	}
	return cfg
}

func applySetupEnvLocked(cfg SetupConfig) {
	os.Setenv("APIG0_PORT", cfg.Port)
	os.Setenv("APIG0_USERS_PATH", cfg.UsersPath)
	os.Setenv("APIG0_SERVICES_PATH", cfg.ServicesPath)
	os.Setenv("APIG0_RATELIMITS_PATH", cfg.RateLimitsPath)
	os.Setenv("VAULT_TYPE", cfg.UserVault.Type)
	os.Setenv("VAULT_SECRET_PATH", cfg.UserVault.SecretPath)
	os.Setenv("VAULT_SECRET_KEY", cfg.UserVault.SecretKey)
	if cfg.UserVault.Address != "" {
		os.Setenv("VAULT_ADDRESS", cfg.UserVault.Address)
	}
	if cfg.UserVault.Engine != "" {
		os.Setenv("VAULT_ENGINE", cfg.UserVault.Engine)
	}
	if cfg.UserVault.FilePath != "" {
		os.Setenv("VAULT_FILE_PATH", cfg.UserVault.FilePath)
	}
}

func resetTargetsLocked() []string {
	candidates := []string{
		setupFilePath,
		"users.json",
		"services.json",
		"ratelimits.json",
		"service-secrets.json",
		"service-secrets.enc.json",
		"totp-secrets.json",
		activeSetup.UsersPath,
		activeSetup.ServicesPath,
		activeSetup.RateLimitsPath,
		activeSetup.UserVault.FilePath,
		activeSetup.ServiceSecrets.FilePath,
		os.Getenv("APIG0_USERS_PATH"),
		os.Getenv("APIG0_SERVICES_PATH"),
		os.Getenv("APIG0_RATELIMITS_PATH"),
		os.Getenv("VAULT_FILE_PATH"),
	}

	out := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		if slices.Contains(out, candidate) {
			continue
		}
		out = append(out, candidate)
	}
	return out
}

// UpgradeToPersistent converts a temporary runtime into a persisted one.
// It writes the current in-memory state (users, services, rate limits, secrets)
// to local files and saves a setup configuration file so the gateway survives
// restarts.
func UpgradeToPersistent(vaultCfg UserVaultSettings, ssCfg ServiceSecretConfig, masterPassword string) error {
	setupMu.Lock()
	defer setupMu.Unlock()

	cfg := activeSetup
	cfg.Mode = SetupModePersistent
	cfg.UsersPath = "users.json"
	cfg.ServicesPath = "services.json"
	cfg.RateLimitsPath = "ratelimits.json"
	cfg.UserVault = vaultCfg
	if cfg.UserVault.Type == "" {
		cfg.UserVault.Type = "file"
	}
	if cfg.UserVault.Type == "file" && cfg.UserVault.FilePath == "" {
		cfg.UserVault.FilePath = "totp-secrets.json"
	}
	if cfg.UserVault.SecretPath == "" {
		cfg.UserVault.SecretPath = "totp"
	}
	if cfg.UserVault.SecretKey == "" {
		cfg.UserVault.SecretKey = "secret"
	}
	cfg.ServiceSecrets = ssCfg
	if cfg.ServiceSecrets.Mode == "" {
		cfg.ServiceSecrets.Mode = ServiceSecretFile
	}
	if cfg.ServiceSecrets.Mode == ServiceSecretFile && cfg.ServiceSecrets.FilePath == "" {
		cfg.ServiceSecrets.FilePath = "service-secrets.json"
	}
	if cfg.ServiceSecrets.Mode == ServiceSecretEncryptedFile && cfg.ServiceSecrets.FilePath == "" {
		cfg.ServiceSecrets.FilePath = "service-secrets.enc.json"
	}

	cfg.RequiresSetup = false
	cfg.Persisted = true
	cfg = normalizeSetupConfig(cfg)

	// Persist the setup config file
	raw, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(setupFilePath, raw, 0600); err != nil {
		return err
	}

	// Save current services to disk
	svcs := GetServiceCatalog()
	if err := SaveServices(svcs); err != nil {
		return err
	}

	// Save current rate limits to disk
	rl := GetRateLimits()
	if err := SaveRateLimits(rl); err != nil {
		return err
	}

	// Save current users to disk (flush the in-memory store to the new file path)
	if store := GetUserStore(); store != nil {
		store.mu.Lock()
		oldPath := store.filePath
		store.filePath = cfg.UsersPath
		err := store.saveToFile()
		if err != nil {
			store.filePath = oldPath
			store.mu.Unlock()
			return err
		}
		store.mu.Unlock()
	}

	activeSetup = cfg
	setupConfigured = true
	applySetupEnvLocked(activeSetup)

	// Re-init vault so TOTP secrets are migrated to the new backend
	if masterPassword == "" {
		masterPassword = os.Getenv("APIG0_SERVICE_MASTER_PASSWORD")
	}

	return nil
}

func randomSuffix() string {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		return "tmp"
	}
	const hex = "0123456789abcdef"
	out := make([]byte, len(b)*2)
	for i, v := range b {
		out[i*2] = hex[v>>4]
		out[i*2+1] = hex[v&0x0f]
	}
	return string(out)
}
