package auth

import (
	"crypto/rand"
	"encoding/base32"
	"fmt"
	"net/http"
	"net/url"

	"apig0/config"

	"github.com/gin-gonic/gin"
)

// POST /api/admin/users
func CreateUserHandler(c *gin.Context) {
	var req struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
		Role     string `json:"role"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Role == "" {
		req.Role = "user"
	}

	raw := make([]byte, 20)
	if _, err := rand.Read(raw); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate secret"})
		return
	}
	secret := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(raw)

	if err := config.StoreUserSecret(req.Username, secret); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "vault error: " + err.Error()})
		return
	}

	if err := config.GetUserStore().Create(req.Username, req.Password, req.Role); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}

	otpauth := fmt.Sprintf(
		"otpauth://totp/Apig0:%s?secret=%s&issuer=Apig0&algorithm=SHA1&digits=6&period=30",
		url.PathEscape(req.Username), secret,
	)
	c.JSON(http.StatusOK, gin.H{"username": req.Username, "role": req.Role, "otpauth": otpauth})
}

// GET /api/admin/users
func ListUsersHandler(c *gin.Context) {
	users := config.GetUserStore().List()
	type safeUser struct {
		Username  string `json:"username"`
		Role      string `json:"role"`
		CreatedAt string `json:"created_at"`
	}
	result := make([]safeUser, 0, len(users))
	for _, u := range users {
		result = append(result, safeUser{
			Username:  u.Username,
			Role:      u.Role,
			CreatedAt: u.CreatedAt.Format("2006-01-02"),
		})
	}
	c.JSON(http.StatusOK, gin.H{"users": result})
}

// DELETE /api/admin/users/:user
func DeleteUserHandler(c *gin.Context) {
	username := c.Param("user")
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
	c.JSON(http.StatusOK, gin.H{"username": username, "otpauth": otpauth})
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
