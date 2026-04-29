package auth

import (
	"net/http"
	"strings"
	"time"

	"apig0/config"

	"github.com/gin-gonic/gin"
)

func apiTokenResponse(token config.APIToken) gin.H {
	out := gin.H{
		"id":                token.ID,
		"name":              token.Name,
		"user":              token.User,
		"key_type":          token.KeyType,
		"token_prefix":      token.TokenPrefix,
		"allowed_services":  token.AllowedServices,
		"openai_service":    token.OpenAIService,
		"allowed_models":    token.AllowedModels,
		"allowed_providers": token.AllowedProviders,
		"rate_limit_rpm":    token.RateLimitRPM,
		"rate_limit_burst":  token.RateLimitBurst,
		"created_at":        token.CreatedAt,
	}
	if !token.ExpiresAt.IsZero() {
		out["expires_at"] = token.ExpiresAt
	}
	if !token.LastUsedAt.IsZero() {
		out["last_used_at"] = token.LastUsedAt
	}
	if !token.RevokedAt.IsZero() {
		out["revoked_at"] = token.RevokedAt
	}
	return out
}

func ListTokensHandler(c *gin.Context) {
	tokens := config.ListAPITokens()
	payload := make([]gin.H, 0, len(tokens))
	for _, token := range tokens {
		payload = append(payload, apiTokenResponse(token))
	}
	c.JSON(http.StatusOK, gin.H{
		"tokens":                 payload,
		"service_catalog":        config.ListServiceNames(),
		"openai_service_catalog": config.ListOpenAICompatibleServiceNames(),
		"node_mode":              config.GetRuntimeStatus().NodeMode,
	})
}

func CreateTokenHandler(c *gin.Context) {
	var req struct {
		Name             string   `json:"name"`
		User             string   `json:"user" binding:"required"`
		KeyType          string   `json:"key_type"`
		AllowedServices  []string `json:"allowed_services"`
		OpenAIService    string   `json:"openai_service"`
		AllowedModels    []string `json:"allowed_models"`
		AllowedProviders []string `json:"allowed_providers"`
		RateLimitRPM     int      `json:"rate_limit_rpm"`
		RateLimitBurst   int      `json:"rate_limit_burst"`
		ExpiresAt        string   `json:"expires_at"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if !config.GetUserStore().Exists(req.User) {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}
	expiresAt, err := parseOptionalTimestamp(req.ExpiresAt)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid expires_at; use RFC3339"})
		return
	}
	raw, token, err := config.CreateAPIToken(config.APITokenCreateParams{
		Name:             req.Name,
		User:             req.User,
		KeyType:          req.KeyType,
		AllowedServices:  req.AllowedServices,
		OpenAIService:    req.OpenAIService,
		AllowedModels:    req.AllowedModels,
		AllowedProviders: req.AllowedProviders,
		RateLimitRPM:     req.RateLimitRPM,
		RateLimitBurst:   req.RateLimitBurst,
		ExpiresAt:        expiresAt,
	})
	if err != nil {
		status := http.StatusInternalServerError
		if strings.Contains(strings.ToLower(err.Error()), "required") || strings.Contains(strings.ToLower(err.Error()), "invalid") {
			status = http.StatusBadRequest
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	delivery, deliveryErr := config.CreatePendingAPITokenDelivery(raw, token, c.GetString("session_user"))
	c.JSON(http.StatusOK, gin.H{
		"ok":             true,
		"token":          apiTokenResponse(token),
		"raw_token":      raw,
		"delivery_ready": deliveryErr == nil,
		"delivery": gin.H{
			"id":           delivery.ID,
			"user":         delivery.User,
			"token_prefix": delivery.TokenPrefix,
			"key_type":     delivery.KeyType,
			"service":      delivery.Service,
			"created_at":   delivery.CreatedAt,
			"expires_at":   delivery.ExpiresAt,
		},
		"delivery_error": func() string {
			if deliveryErr == nil {
				return ""
			}
			return deliveryErr.Error()
		}(),
	})
}

func RevokeTokenHandler(c *gin.Context) {
	if err := config.RevokeAPIToken(c.Param("id")); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "token not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func GetUserPoliciesHandler(c *gin.Context) {
	user := strings.TrimSpace(c.Param("user"))
	if !config.GetUserStore().Exists(user) {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"user":            user,
		"policies":        config.GetUserAccessPolicies(user),
		"service_catalog": config.ListServiceNames(),
	})
}

func UpdateUserPoliciesHandler(c *gin.Context) {
	user := strings.TrimSpace(c.Param("user"))
	if !config.GetUserStore().Exists(user) {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}
	if config.GetUserStore().GetRole(user) == "admin" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "admin accounts do not use route policies"})
		return
	}
	var req struct {
		Policies []config.AccessPolicyRule `json:"policies"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := config.SetUserAccessPolicies(user, req.Policies); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"ok":       true,
		"user":     user,
		"policies": config.GetUserAccessPolicies(user),
	})
}

func ListAuditHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"events":   config.ListRecentAuditEvents(250),
		"counters": config.GetAuditCounters(),
		"log_path": config.AuditLogPath(),
	})
}

func parseOptionalTimestamp(raw string) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, nil
	}
	return time.Parse(time.RFC3339, raw)
}
