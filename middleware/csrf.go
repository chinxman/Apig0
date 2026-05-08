package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"

	"apig0/auth"

	"github.com/gin-gonic/gin"
)

const csrfTokenHeader = "X-CSRF-Token"
const csrfCookieName = "apig0_csrf"

func newCSRFCookie(value string) *http.Cookie {
	cookie := &http.Cookie{
		Name:     csrfCookieName,
		Value:    value,
		Path:     "/",
		MaxAge:   86400,
		Secure:   true,
		HttpOnly: false, // JS must read the cookie for the double-submit header flow.
		SameSite: http.SameSiteStrictMode,
	}
	if !auth.IsSecure() {
		cookie.Secure = false
	}
	return cookie
}

// CSRF implements double-submit cookie pattern.
// On every response, a random CSRF token is set as a non-HttpOnly cookie
// (so JS can read it). On state-changing requests (POST/PUT/DELETE/PATCH),
// the client must send the cookie value back in the X-CSRF-Token header.
// Safe methods (GET/HEAD/OPTIONS) and the login endpoints are exempt.
func CSRF() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Always set/refresh the CSRF cookie so the client has a token
		tok, err := c.Cookie(csrfCookieName)
		if err != nil || tok == "" {
			tok = csrfRandHex(32)
		}
		// Non-HttpOnly so JS can read it and mirror it into X-CSRF-Token.
		http.SetCookie(c.Writer, newCSRFCookie(tok))

		// Safe methods — no check needed
		switch c.Request.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			c.Next()
			return
		}
		if source, _ := c.Get("auth_source"); source == "token" {
			c.Next()
			return
		}

		// Validate: header must match cookie
		headerTok := c.GetHeader(csrfTokenHeader)
		if headerTok == "" || headerTok != tok {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "CSRF token missing or invalid"})
			return
		}

		c.Next()
	}
}

func csrfRandHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}
