package auth

import (
	"errors"
	"net/http"
	"os"
	"strings"

	"apig0/config"
	"apig0/proxy"

	"github.com/gin-gonic/gin"
)

type serviceAdminRequest struct {
	Name          string `json:"name"`
	BaseURL       string `json:"base_url" binding:"required"`
	AuthType      string `json:"auth_type"`
	HeaderName    string `json:"header_name"`
	BasicUsername string `json:"basic_username"`
	TLSSkipVerify *bool  `json:"tls_skip_verify"`
	Provider      string `json:"provider"`
	OpenAICompat  bool   `json:"openai_compatible"`
	TimeoutMS     *int   `json:"timeout_ms"`
	RetryCount    *int   `json:"retry_count"`
	Secret        string `json:"secret"`
	SecretNotes   string `json:"secret_notes"`
	SecretExpires string `json:"secret_expires_at"`
	ClearSecret   bool   `json:"clear_secret"`
	Enabled       *bool  `json:"enabled"`
}

func ListServicesHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"services":               config.GetServiceCatalog(),
		"service_catalog":        config.ListServiceNames(),
		"service_secret_storage": config.ServiceSecretStatus(),
		"service_secret_meta":    config.ListServiceSecretMetadata(),
	})
}

func CreateServiceHandler(c *gin.Context) {
	var req serviceAdminRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid service payload"})
		return
	}

	cfg, name, err := buildServiceConfig(req, nil)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if _, exists := config.GetServiceConfig(name); exists {
		c.JSON(http.StatusConflict, gin.H{"error": "service already exists"})
		return
	}
	if err := ensureServiceSecretWritable(req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := config.UpsertService(cfg); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if req.Secret != "" {
		if err := config.SetServiceSecret(name, strings.TrimSpace(req.Secret)); err != nil {
			_ = config.DeleteService(name)
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if expiresAt, err := parseOptionalTimestamp(req.SecretExpires); err == nil {
			_ = config.NoteServiceSecretRotated(name, expiresAt, req.SecretNotes)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"ok":                     true,
		"service":                cfg,
		"service_catalog":        config.ListServiceNames(),
		"service_secret_storage": config.ServiceSecretStatus(),
		"service_secret_meta":    config.ListServiceSecretMetadata(),
	})
}

func UpdateServiceHandler(c *gin.Context) {
	name := config.NormalizeAllowedServiceName(c.Param("name"))
	existing, ok := config.GetServiceConfig(name)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "service not found"})
		return
	}

	var req serviceAdminRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid service payload"})
		return
	}

	if trimmed := config.NormalizeAllowedServiceName(req.Name); trimmed != "" && trimmed != name {
		c.JSON(http.StatusBadRequest, gin.H{"error": "renaming services is not supported yet"})
		return
	}
	if err := ensureServiceSecretWritable(req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	req.Name = name
	cfg, _, err := buildServiceConfig(req, &existing)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := config.UpsertService(cfg); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if req.ClearSecret {
		if err := config.DeleteServiceSecret(name); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		_ = config.DeleteServiceSecretMetadata(name)
	}
	if req.Secret != "" {
		if err := config.SetServiceSecret(name, strings.TrimSpace(req.Secret)); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if expiresAt, err := parseOptionalTimestamp(req.SecretExpires); err == nil {
			_ = config.NoteServiceSecretRotated(name, expiresAt, req.SecretNotes)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"ok":                     true,
		"service":                cfg,
		"service_catalog":        config.ListServiceNames(),
		"service_secret_storage": config.ServiceSecretStatus(),
		"service_secret_meta":    config.ListServiceSecretMetadata(),
	})
}

func DeleteServiceHandler(c *gin.Context) {
	name := config.NormalizeAllowedServiceName(c.Param("name"))
	existing, ok := config.GetServiceConfig(name)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "service not found"})
		return
	}

	if existing.HasSecret && config.ServiceSecretStatus().Locked {
		c.JSON(http.StatusBadRequest, gin.H{"error": "service secret storage is locked; unlock it before deleting a saved key"})
		return
	}
	if existing.HasSecret {
		if err := config.DeleteServiceSecret(name); err != nil && !errors.Is(err, config.ErrServiceSecretsLocked) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		_ = config.DeleteServiceSecretMetadata(name)
	}
	if err := config.DeleteService(name); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			c.JSON(http.StatusNotFound, gin.H{"error": "service not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if store := config.GetUserStore(); store != nil {
		if err := store.RemoveAllowedServiceEverywhere(name); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"ok":                     true,
		"service_catalog":        config.ListServiceNames(),
		"service_secret_storage": config.ServiceSecretStatus(),
		"service_secret_meta":    config.ListServiceSecretMetadata(),
	})
}

func TestServiceAuthHandler(c *gin.Context) {
	name := config.NormalizeAllowedServiceName(c.Param("name"))
	service, ok := config.GetServiceConfig(name)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "service not found"})
		return
	}
	var req struct {
		Path string `json:"path"`
	}
	_ = c.ShouldBindJSON(&req)
	status, err := proxy.TestServiceAuth(service, req.Path)
	if err != nil {
		_ = config.NoteServiceSecretTest(name, http.StatusBadGateway)
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	_ = config.NoteServiceSecretTest(name, status)
	c.JSON(http.StatusOK, gin.H{
		"ok":                  status < 500,
		"status":              status,
		"service_secret_meta": config.ListServiceSecretMetadata(),
	})
}

func buildServiceConfig(req serviceAdminRequest, existing *config.ServiceConfig) (config.ServiceConfig, string, error) {
	name := config.NormalizeAllowedServiceName(req.Name)
	if existing != nil && name == "" {
		name = existing.Name
	}
	if name == "" {
		return config.ServiceConfig{}, "", errors.New("service name is required")
	}

	baseURL := strings.TrimSpace(req.BaseURL)
	if baseURL == "" {
		return config.ServiceConfig{}, "", errors.New("base URL is required")
	}
	if err := config.ValidateServiceBaseURL(baseURL); err != nil {
		return config.ServiceConfig{}, "", err
	}

	enabled := true
	if existing != nil {
		enabled = existing.Enabled
	}
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	cfg := config.ServiceConfig{
		Name:          name,
		BaseURL:       baseURL,
		AuthType:      config.ServiceAuthType(strings.TrimSpace(req.AuthType)),
		HeaderName:    strings.TrimSpace(req.HeaderName),
		BasicUsername: strings.TrimSpace(req.BasicUsername),
		Provider:      config.NormalizeProviderName(req.Provider),
		OpenAICompat:  req.OpenAICompat,
		Enabled:       enabled,
	}
	if existing != nil {
		cfg.TLSSkipVerify = existing.TLSSkipVerify
		cfg.TimeoutMS = existing.TimeoutMS
		cfg.RetryCount = existing.RetryCount
	}
	if req.TLSSkipVerify != nil {
		cfg.TLSSkipVerify = *req.TLSSkipVerify
	}
	if req.TimeoutMS != nil {
		cfg.TimeoutMS = *req.TimeoutMS
	}
	if req.RetryCount != nil {
		cfg.RetryCount = *req.RetryCount
	}
	if existing != nil {
		cfg.HasSecret = existing.HasSecret
	}
	if strings.TrimSpace(req.Secret) != "" {
		cfg.HasSecret = true
	}
	if req.ClearSecret {
		cfg.HasSecret = false
	}

	return cfg, name, nil
}

func ensureServiceSecretWritable(req serviceAdminRequest) error {
	if strings.TrimSpace(req.Secret) == "" && !req.ClearSecret {
		return nil
	}
	if config.ServiceSecretStatus().Locked {
		return errors.New("service secret storage is locked; unlock it before changing saved keys")
	}
	return nil
}
