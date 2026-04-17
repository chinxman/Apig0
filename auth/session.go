package auth

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"apig0/config"

	"github.com/gin-gonic/gin"
)

type sessionEntry struct {
	user    string
	expires time.Time
}

var (
	mu         sync.Mutex
	challenges = map[string]sessionEntry{}
	sessions   = map[string]sessionEntry{}
)

func init() {
	// Sweep expired sessions and challenges every 60 seconds.
	go func() {
		ticker := time.NewTicker(60 * time.Second)
		for range ticker.C {
			sweepExpired()
		}
	}()
}

func sweepExpired() {
	now := time.Now()
	mu.Lock()
	defer mu.Unlock()
	for k, e := range challenges {
		if now.After(e.expires) {
			delete(challenges, k)
		}
	}
	for k, e := range sessions {
		if now.After(e.expires) {
			delete(sessions, k)
		}
	}
}

// SessionTTL returns the configured session lifetime.
// Reads APIG0_SESSION_TTL as a Go duration string (e.g. "8h", "30m", "24h").
// Defaults to 8 hours if unset or unparseable.
func SessionTTL() time.Duration {
	if raw := os.Getenv("APIG0_SESSION_TTL"); raw != "" {
		if d, err := time.ParseDuration(raw); err == nil && d > 0 {
			return d
		}
	}
	return 8 * time.Hour
}

// NewChallenge stores a short-lived (5 min) token tied to user after password is validated.
func NewChallenge(user string) string {
	tok := randHex(32)
	mu.Lock()
	challenges[tok] = sessionEntry{user: user, expires: time.Now().Add(5 * time.Minute)}
	mu.Unlock()
	return tok
}

// PeekChallenge returns the user for a challenge without consuming it.
// Returns ("", false) if missing or expired.
func PeekChallenge(id string) (string, bool) {
	mu.Lock()
	defer mu.Unlock()
	e, ok := challenges[id]
	if !ok || time.Now().After(e.expires) {
		delete(challenges, id)
		return "", false
	}
	return e.user, true
}

// ConsumeChallenge deletes the challenge — call this only after successful TOTP.
func ConsumeChallenge(id string) {
	mu.Lock()
	delete(challenges, id)
	mu.Unlock()
}

// NewSession creates a session token valid for SessionTTL() and returns it.
func NewSession(user string) string {
	tok := randHex(32)
	mu.Lock()
	sessions[tok] = sessionEntry{user: user, expires: time.Now().Add(SessionTTL())}
	mu.Unlock()
	return tok
}

// ValidateSession returns the user for a valid, non-expired session token.
func ValidateSession(tok string) (string, bool) {
	mu.Lock()
	defer mu.Unlock()
	e, ok := sessions[tok]
	if !ok || time.Now().After(e.expires) {
		delete(sessions, tok)
		return "", false
	}
	return e.user, true
}

// DeleteSession removes a session immediately (logout).
func DeleteSession(tok string) {
	mu.Lock()
	delete(sessions, tok)
	mu.Unlock()
}

func ResetSessionState() {
	mu.Lock()
	defer mu.Unlock()
	challenges = map[string]sessionEntry{}
	sessions = map[string]sessionEntry{}
}

// SessionMiddleware rejects requests without a valid apig0_session cookie.
// Sets "session_user" in the Gin context on success.
func SessionMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		tok, err := c.Cookie("apig0_session")
		if err != nil || tok == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
			return
		}
		user, ok := ValidateSession(tok)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "session expired"})
			return
		}
		c.Set("session_user", user)
		c.Next()
	}
}

// AdminMiddleware requires a valid session AND admin role.
// Returns 403 if the user is authenticated but not an admin.
func AdminMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		tok, err := c.Cookie("apig0_session")
		if err != nil || tok == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
			return
		}
		user, ok := ValidateSession(tok)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "session expired"})
			return
		}
		c.Set("session_user", user)

		if config.GetUserStore() != nil && config.GetUserStore().GetRole(user) != "admin" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "admin access required"})
			return
		}
		c.Next()
	}
}

// IsSecure returns true when the gateway should set Secure cookies.
// Looks at APIG0_SECURE env var ("true"/"false"). Defaults to false
// since TLS is not yet configured — set APIG0_SECURE=true once TLS is added.
func IsSecure() bool {
	if v := os.Getenv("APIG0_SECURE"); strings.EqualFold(v, "true") {
		return true
	}
	return false
}

// SetSessionCookie writes the session cookie with Secure + SameSite=Strict.
func SetSessionCookie(c *gin.Context, tok string) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     "apig0_session",
		Value:    tok,
		Path:     "/",
		MaxAge:   int(SessionTTL().Seconds()),
		Secure:   IsSecure(),
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})
}

// ClearSessionCookie removes the session cookie.
func ClearSessionCookie(c *gin.Context) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     "apig0_session",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		Secure:   IsSecure(),
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})
}

func randHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}
