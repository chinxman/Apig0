package auth

import (
	"net/http"

	"apig0/config"

	"github.com/gin-gonic/gin"
)

// LoginHandler validates username + password and returns a short-lived challenge token.
// POST /auth/login  {"username":"...","password":"..."}
func LoginHandler(c *gin.Context) {
	var req struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "username and password required"})
		return
	}

	if !config.ValidatePassword(req.Username, req.Password) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"challenge": NewChallenge(req.Username)})
}

// VerifyHandler validates the challenge token + TOTP code and sets a session cookie.
// POST /auth/verify  {"challenge":"...","code":"..."}
func VerifyHandler(c *gin.Context) {
	var req struct {
		Challenge string `json:"challenge" binding:"required"`
		Code      string `json:"code"      binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "challenge and code required"})
		return
	}

	// Peek — check challenge exists without consuming it yet
	user, ok := PeekChallenge(req.Challenge)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired challenge"})
		return
	}

	secret := config.LoadUserSecret(user)
	if secret == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "no TOTP secret configured for user"})
		return
	}

	// Validate TOTP before consuming — wrong code leaves the challenge intact
	if !ValidateTOTP(user, req.Code, secret) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid TOTP code"})
		return
	}

	// Only consume the challenge once the code is confirmed correct
	ConsumeChallenge(req.Challenge)

	tok := NewSession(user)
	maxAge := int(SessionTTL().Seconds())
	c.SetCookie("apig0_session", tok, maxAge, "/", "", false, true)
	c.JSON(http.StatusOK, gin.H{"ok": true, "user": user})
}

// LogoutHandler clears the session cookie and removes the server-side session.
// POST /auth/logout
func LogoutHandler(c *gin.Context) {
	if tok, err := c.Cookie("apig0_session"); err == nil {
		DeleteSession(tok)
	}
	c.SetCookie("apig0_session", "", -1, "/", "", false, true)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
