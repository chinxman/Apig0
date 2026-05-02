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
	Type        string            `json:"type"`
	Address     string            `json:"address,omitempty"`
	Engine      string            `json:"engine,omitempty"`
	SecretPath  string            `json:"secret_path,omitempty"`
	SecretKey   string            `json:"secret_key,omitempty"`
	FilePath    string            `json:"file_path,omitempty"`
	ProviderEnv map[string]string `json:"provider_env,omitempty"`
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
		Port:           "8989",
		UsersPath:      filepath.Join(os.TempDir(), "apig0-users-"+randomSuffix()+".json"),
		ServicesPath:   filepath.Join(os.TempDir(), "apig0-services-"+randomSuffix()+".json"),
		RateLimitsPath: filepath.Join(os.TempDir(), "apig0-ratelimits-"+randomSuffix()+".json"),
		UserVault: UserVaultSettings{
			Type:        "env",
			SecretPath:  "totp",
			SecretKey:   "secret",
			ProviderEnv: map[string]string{},
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

	ResetAuditState()
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
		cfg.Port = "8989"
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
	cfg.UserVault.ProviderEnv = normalizeProviderEnv(cfg.UserVault.ProviderEnv)
	if cfg.UserVault.Type == "file" && cfg.UserVault.FilePath == "" {
		cfg.UserVault.FilePath = "totp-secrets.json"
	}
	if cfg.Mode == SetupModePersistent {
		cfg.ServiceSecrets = NormalizePersistentServiceSecretConfig(cfg.ServiceSecrets)
	} else {
		cfg.ServiceSecrets = NormalizeTemporaryServiceSecretConfig(cfg.ServiceSecrets)
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
	} else {
		os.Unsetenv("VAULT_ADDRESS")
	}
	if cfg.UserVault.Engine != "" {
		os.Setenv("VAULT_ENGINE", cfg.UserVault.Engine)
	} else {
		os.Unsetenv("VAULT_ENGINE")
	}
	if cfg.UserVault.FilePath != "" {
		os.Setenv("VAULT_FILE_PATH", cfg.UserVault.FilePath)
	} else {
		os.Unsetenv("VAULT_FILE_PATH")
	}
	applyProviderEnvLocked(cfg.UserVault.ProviderEnv)
}

var providerEnvKeys = []string{
	"AWS_REGION",
	"AWS_DEFAULT_REGION",
	"AWS_PROFILE",
	"AWS_ACCESS_KEY_ID",
	"AWS_SECRET_ACCESS_KEY",
	"AWS_SESSION_TOKEN",
	"GCP_PROJECT",
	"CLOUDSDK_AUTH_CREDENTIAL_FILE_OVERRIDE",
	"AZURE_VAULT_NAME",
	"AZURE_TENANT_ID",
	"AZURE_CLIENT_ID",
	"AZURE_CLIENT_SECRET",
	"OP_VAULT",
	"OP_SERVICE_ACCOUNT_TOKEN",
	"CYBERARK_ADDRESS",
	"CYBERARK_APP_ID",
	"CYBERARK_SAFE",
	"CYBERARK_FOLDER",
	"VAULT_HTTP_URL",
	"VAULT_HTTP_METHOD",
	"VAULT_HTTP_HEADER",
	"VAULT_HTTP_JSON_PATH",
	"VAULT_HTTP_BODY",
	"VAULT_HTTP_BASE64",
	"VAULT_EXEC_COMMAND",
}

func normalizeProviderEnv(env map[string]string) map[string]string {
	out := map[string]string{}
	for _, key := range providerEnvKeys {
		if strings.TrimSpace(env[key]) != "" {
			out[key] = strings.TrimSpace(env[key])
		}
	}
	if region := out["AWS_REGION"]; region != "" && out["AWS_DEFAULT_REGION"] == "" {
		out["AWS_DEFAULT_REGION"] = region
	}
	return out
}

func applyProviderEnvLocked(env map[string]string) {
	env = normalizeProviderEnv(env)
	for _, key := range providerEnvKeys {
		if value := env[key]; value != "" {
			os.Setenv(key, value)
		} else {
			os.Unsetenv(key)
		}
	}
}

func resetTargetsLocked() []string {
	candidates := []string{
		setupFilePath,
		"users.json",
		"services.json",
		"ratelimits.json",
		"api-tokens.json",
		"api-token-deliveries.json",
		"access-policies.json",
		"service-secret-metadata.json",
		"audit.log",
		apiTokensFilePathForSetupLocked(),
		pendingTokenDeliveryFilePathForSetupLocked(),
		accessPoliciesFilePathForSetupLocked(),
		serviceSecretMetadataPathForSetupLocked(),
		auditFilePathForSetupLocked(),
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

func apiTokensFilePathForSetupLocked() string {
	if path := strings.TrimSpace(os.Getenv("APIG0_API_TOKENS_PATH")); path != "" {
		return path
	}
	if activeSetup.Mode == SetupModePersistent {
		return "api-tokens.json"
	}
	return apiTokenPath
}

func pendingTokenDeliveryFilePathForSetupLocked() string {
	if path := strings.TrimSpace(os.Getenv("APIG0_TOKEN_DELIVERIES_PATH")); path != "" {
		return path
	}
	if activeSetup.Mode == SetupModePersistent {
		return "api-token-deliveries.json"
	}
	return pendingTokenDeliveryPath
}

func accessPoliciesFilePathForSetupLocked() string {
	if path := strings.TrimSpace(os.Getenv("APIG0_ACCESS_POLICIES_PATH")); path != "" {
		return path
	}
	if activeSetup.Mode == SetupModePersistent {
		return "access-policies.json"
	}
	return accessPolicyPath
}

func serviceSecretMetadataPathForSetupLocked() string {
	if path := strings.TrimSpace(os.Getenv("APIG0_SERVICE_SECRET_METADATA_PATH")); path != "" {
		return path
	}
	if activeSetup.Mode == SetupModePersistent {
		return "service-secret-metadata.json"
	}
	return serviceSecretMetaPath
}

func auditFilePathForSetupLocked() string {
	if path := strings.TrimSpace(os.Getenv("APIG0_AUDIT_LOG_PATH")); path != "" {
		return path
	}
	if activeSetup.Mode == SetupModePersistent {
		return "audit.log"
	}
	return auditLogPath
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
	cfg.ServiceSecrets = NormalizePersistentServiceSecretConfig(ssCfg)

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
