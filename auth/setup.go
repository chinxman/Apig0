package auth

import (
	"net/http"
	"os"
	"strings"

	"apig0/config"

	"github.com/gin-gonic/gin"
)

type setupServiceRequest struct {
	Name          string `json:"name"`
	BaseURL       string `json:"base_url"`
	AuthType      string `json:"auth_type"`
	HeaderName    string `json:"header_name"`
	BasicUsername string `json:"basic_username"`
	Secret        string `json:"secret"`
	Enabled       bool   `json:"enabled"`
}

// GET /api/setup/status
func SetupStatusHandler(c *gin.Context) {
	c.JSON(http.StatusOK, config.GetRuntimeStatus())
}

// POST /api/setup/reset
func ResetSetupHandler(c *gin.Context) {
	if err := config.ResetSetupState(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ResetSessionState()
	if err := config.ReloadRuntime(nil, ""); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "status": config.GetRuntimeStatus()})
		return
	}
	ClearSessionCookie(c)
	c.JSON(http.StatusOK, gin.H{"ok": true, "status": config.GetRuntimeStatus()})
}

// POST /api/setup/complete
func CompleteSetupHandler(c *gin.Context) {
	status := config.GetRuntimeStatus()
	// Block only when setup is done AND an admin already exists.
	// If setup ran before but all users were lost (e.g. temporary mode restart),
	// allow re-setup so the admin can be recreated without wiping service config.
	if !status.SetupRequired && status.HasAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "setup already completed", "status": status})
		return
	}

	var req struct {
		Mode           string                     `json:"mode"`
		Port           string                     `json:"port"`
		AdminUsername  string                     `json:"admin_username" binding:"required"`
		AdminPassword  string                     `json:"admin_password" binding:"required"`
		UserVault      config.UserVaultSettings   `json:"user_vault"`
		ServiceSecrets config.ServiceSecretConfig `json:"service_secrets"`
		ServiceMaster  string                     `json:"service_master_password"`
		Services       []setupServiceRequest      `json:"services"`
		EnableDemo     bool                       `json:"enable_demo"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid setup payload"})
		return
	}

	mode := config.SetupMode(strings.TrimSpace(req.Mode))
	if mode == "" {
		mode = config.SetupModeTemporary
	}
	setupCfg := config.SetupConfig{
		Mode:           mode,
		Port:           strings.TrimSpace(req.Port),
		UserVault:      req.UserVault,
		ServiceSecrets: req.ServiceSecrets,
	}

	services, secrets := normalizeSetupServices(req.Services, req.EnableDemo)
	if len(services) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "at least one service or demo mode is required"})
		return
	}

	if mode == config.SetupModePersistent {
		setupCfg.UsersPath = "users.json"
		setupCfg.ServicesPath = "services.json"
		setupCfg.RateLimitsPath = "ratelimits.json"
		if setupCfg.ServiceSecrets.Mode == config.ServiceSecretFile {
			setupCfg.ServiceSecrets.FilePath = "service-secrets.json"
		}
		if setupCfg.ServiceSecrets.Mode == config.ServiceSecretEncryptedFile {
			setupCfg.ServiceSecrets.FilePath = "service-secrets.enc.json"
		}
		if setupCfg.UserVault.Type == "file" && setupCfg.UserVault.FilePath == "" {
			setupCfg.UserVault.FilePath = "totp-secrets.json"
		}
		if err := config.SavePersistentSetup(setupCfg); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if err := config.SaveServices(services); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	} else {
		if setupCfg.ServiceSecrets.Mode == "" {
			setupCfg.ServiceSecrets.Mode = config.ServiceSecretMemory
		}
		if setupCfg.UserVault.Type == "" {
			setupCfg.UserVault.Type = "env"
		}
		config.ActivateTemporarySetup(setupCfg)
		config.SetServicesInMemory(services)
	}

	if req.ServiceMaster != "" {
		_ = os.Setenv("APIG0_SERVICE_MASTER_PASSWORD", req.ServiceMaster)
	}
	if err := config.ReloadRuntime(secrets, req.ServiceMaster); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	_, otpauth, err := provisionUser(req.AdminUsername, req.AdminPassword, "admin", nil, false)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "status": config.GetRuntimeStatus()})
		return
	}

	tok := NewSession(req.AdminUsername)
	SetSessionCookie(c, tok)
	c.JSON(http.StatusOK, gin.H{
		"ok":           true,
		"user":         req.AdminUsername,
		"role":         "admin",
		"otpauth":      otpauth,
		"qr":           GenerateQRDataURI(otpauth),
		"status":       config.GetRuntimeStatus(),
		"bootstrapped": true,
	})
}

// POST /api/setup/bootstrap-admin
func BootstrapAdminHandler(c *gin.Context) {
	var req struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "username and password required"})
		return
	}

	if config.GetRuntimeStatus().SetupRequired {
		c.JSON(http.StatusBadRequest, gin.H{"error": "complete initial setup first", "status": config.GetRuntimeStatus()})
		return
	}
	if config.GetRuntimeStatus().HasAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "bootstrap disabled: admin already exists", "status": config.GetRuntimeStatus()})
		return
	}

	_, otpauth, err := provisionUser(req.Username, req.Password, "admin", nil, false)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	tok := NewSession(req.Username)
	SetSessionCookie(c, tok)
	c.JSON(http.StatusOK, gin.H{
		"ok":           true,
		"user":         req.Username,
		"role":         "admin",
		"otpauth":      otpauth,
		"qr":           GenerateQRDataURI(otpauth),
		"status":       config.GetRuntimeStatus(),
		"bootstrapped": true,
	})
}

// POST /api/admin/settings/storage — upgrade from temporary to persistent
func UpgradeStorageHandler(c *gin.Context) {
	status := config.GetRuntimeStatus()
	if status.PersistentConfigured {
		c.JSON(http.StatusBadRequest, gin.H{"error": "already in persistent mode", "status": status})
		return
	}

	var req struct {
		UserVault      config.UserVaultSettings   `json:"user_vault"`
		ServiceSecrets config.ServiceSecretConfig `json:"service_secrets"`
		MasterPassword string                     `json:"master_password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}

	if req.ServiceSecrets.Mode == config.ServiceSecretEncryptedFile && req.MasterPassword == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "master password required for encrypted file mode"})
		return
	}

	if req.MasterPassword != "" {
		_ = os.Setenv("APIG0_SERVICE_MASTER_PASSWORD", req.MasterPassword)
	}

	// Snapshot TOTP secrets BEFORE upgrade — InitSecrets will clear the in-memory map
	savedSecrets := make(map[string]string, len(config.UserSecrets))
	for k, v := range config.UserSecrets {
		savedSecrets[k] = v
	}

	if err := config.UpgradeToPersistent(req.UserVault, req.ServiceSecrets, req.MasterPassword); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Re-init secrets with the new vault backend (file, hashicorp, etc.)
	config.InitSecrets()

	// Migrate TOTP secrets into the new vault backend
	for username, secret := range savedSecrets {
		if secret != "" {
			_ = config.StoreUserSecret(username, secret)
		}
	}

	// Persist service secrets to the new backend
	if err := config.ReloadRuntime(nil, req.MasterPassword); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "reload failed: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true, "status": config.GetRuntimeStatus()})
}

func normalizeSetupServices(in []setupServiceRequest, enableDemo bool) ([]config.ServiceConfig, map[string]string) {
	services := make([]config.ServiceConfig, 0, len(in))
	secrets := make(map[string]string)
	for _, svc := range in {
		name := strings.TrimSpace(strings.ToLower(svc.Name))
		baseURL := strings.TrimSpace(svc.BaseURL)
		if name == "" || baseURL == "" {
			continue
		}
		cfg := config.ServiceConfig{
			Name:          name,
			BaseURL:       baseURL,
			AuthType:      config.ServiceAuthType(strings.TrimSpace(svc.AuthType)),
			HeaderName:    strings.TrimSpace(svc.HeaderName),
			BasicUsername: strings.TrimSpace(svc.BasicUsername),
			Enabled:       true,
			HasSecret:     strings.TrimSpace(svc.Secret) != "",
		}
		services = append(services, cfg)
		if strings.TrimSpace(svc.Secret) != "" {
			secrets[name] = strings.TrimSpace(svc.Secret)
		}
	}
	if len(services) == 0 && enableDemo {
		for _, demo := range config.DefaultDemoServices() {
			services = append(services, demo)
		}
	}
	return services, secrets
}
