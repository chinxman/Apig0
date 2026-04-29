package auth

import (
	"crypto/rand"
	"encoding/base32"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"apig0/config"

	"github.com/gin-gonic/gin"
)

func ProvisionUser(username, password, role string, allowedServices []string, restrictServices bool) (string, string, error) {
	raw := make([]byte, 20)
	if _, err := rand.Read(raw); err != nil {
		return "", "", fmt.Errorf("failed to generate secret")
	}
	secret := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(raw)

	if err := config.GetUserStore().Create(username, password, role, allowedServices, restrictServices); err != nil {
		return "", "", err
	}
	if err := config.StoreUserSecret(username, secret); err != nil {
		_ = config.GetUserStore().Delete(username)
		return "", "", fmt.Errorf("vault error: %w", err)
	}

	otpauth := fmt.Sprintf(
		"otpauth://totp/Apig0:%s?secret=%s&issuer=Apig0&algorithm=SHA1&digits=6&period=30",
		url.PathEscape(username), secret,
	)
	return secret, otpauth, nil
}

// POST /api/admin/users
func CreateUserHandler(c *gin.Context) {
	var req struct {
		Username        string   `json:"username" binding:"required"`
		Password        string   `json:"password" binding:"required"`
		Role            string   `json:"role"`
		AllowedServices []string `json:"allowed_services"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Role == "" {
		req.Role = "user"
	}

	allowedServices := config.NormalizeAllowedServices(req.AllowedServices)
	restrictServices := req.Role != "admin"
	_, otpauth, err := ProvisionUser(req.Username, req.Password, req.Role, allowedServices, restrictServices)
	if err != nil {
		status := http.StatusInternalServerError
		if strings.Contains(err.Error(), "already exists") {
			status = http.StatusConflict
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"username":                  req.Username,
		"role":                      req.Role,
		"service_access_configured": restrictServices,
		"allowed_services":          allowedServices,
		"otpauth":                   otpauth,
		"qr":                        GenerateQRDataURI(otpauth),
	})
}

// GET /api/admin/users
func ListUsersHandler(c *gin.Context) {
	users := config.GetUserStore().List()
	type safeUser struct {
		Username                string   `json:"username"`
		Role                    string   `json:"role"`
		ProtectedAdmin          bool     `json:"protected_admin,omitempty"`
		ServiceAccessConfigured bool     `json:"service_access_configured"`
		AllowedServices         []string `json:"allowed_services"`
		CreatedAt               string   `json:"created_at"`
	}
	result := make([]safeUser, 0, len(users))
	for _, u := range users {
		result = append(result, safeUser{
			Username:                u.Username,
			Role:                    u.Role,
			ProtectedAdmin:          config.GetUserStore().IsProtectedAdmin(u.Username),
			ServiceAccessConfigured: u.ServiceAccessConfigured,
			AllowedServices:         config.NormalizeAllowedServices(u.AllowedServices),
			CreatedAt:               u.CreatedAt.Format("2006-01-02"),
		})
	}
	c.JSON(http.StatusOK, gin.H{
		"users":           result,
		"service_catalog": config.ListServiceNames(),
	})
}

// PUT /api/admin/users/:user/access
func UpdateUserAccessHandler(c *gin.Context) {
	username := c.Param("user")
	if !config.GetUserStore().Exists(username) {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	var req struct {
		AllowedServices []string `json:"allowed_services"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if config.GetUserStore().GetRole(username) == "admin" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "admin accounts always have full access"})
		return
	}

	allowedServices := config.NormalizeAllowedServices(req.AllowedServices)
	if err := config.GetUserStore().SetAllowedServices(username, allowedServices); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"ok":                        true,
		"service_access_configured": true,
		"allowed_services":          allowedServices,
	})
}

// DELETE /api/admin/users/:user
func DeleteUserHandler(c *gin.Context) {
	username := c.Param("user")
	if config.GetUserStore().IsProtectedAdmin(username) {
		c.JSON(http.StatusForbidden, gin.H{"error": "the first admin can only be removed by a full reset"})
		return
	}
	if err := config.GetUserStore().Delete(username); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	config.DeleteUserSecret(username)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// POST /api/admin/users/:user/reset
func ResetTOTPHandler(c *gin.Context) {
	username := c.Param("user")
	if !config.GetUserStore().Exists(username) {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	raw := make([]byte, 20)
	if _, err := rand.Read(raw); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate secret"})
		return
	}
	secret := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(raw)

	if err := config.StoreUserSecret(username, secret); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "vault error: " + err.Error()})
		return
	}

	otpauth := fmt.Sprintf(
		"otpauth://totp/Apig0:%s?secret=%s&issuer=Apig0&algorithm=SHA1&digits=6&period=30",
		url.PathEscape(username), secret,
	)
	c.JSON(http.StatusOK, gin.H{
		"username": username,
		"otpauth":  otpauth,
		"qr":       GenerateQRDataURI(otpauth),
	})
}

// GET /api/admin/settings/ratelimits
func GetRateLimitsHandler(c *gin.Context) {
	c.JSON(http.StatusOK, config.GetRateLimits())
}

// POST /api/admin/settings/ratelimits
func SaveRateLimitsHandler(c *gin.Context) {
	var settings config.RateLimitSettings
	if err := c.ShouldBindJSON(&settings); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := config.SaveRateLimits(settings); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
