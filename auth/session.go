package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"net"
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

func authenticateRequest(c *gin.Context) (string, string, bool) {
	if raw := extractBearerToken(c.GetHeader("Authorization")); raw != "" {
		if token, ok := config.ValidateAPIToken(raw); ok {
			attachAPITokenContext(c, token)
			return token.User, "token", true
		}
	}
	if raw := strings.TrimSpace(c.GetHeader("X-API-Key")); raw != "" {
		if token, ok := config.ValidateAPIToken(raw); ok {
			attachAPITokenContext(c, token)
			return token.User, "token", true
		}
	}

	tok, err := c.Cookie("apig0_session")
	if err != nil || tok == "" {
		return "", "", false
	}
	user, ok := ValidateSession(tok)
	if !ok {
		return "", "", false
	}
	c.Set("session_user", user)
	c.Set("auth_source", "session")
	return user, "session", true
}

func attachAPITokenContext(c *gin.Context, token config.APIToken) {
	c.Set("session_user", token.User)
	c.Set("auth_source", "token")
	c.Set("api_token", token)
	c.Set("api_token_id", token.ID)
	c.Set("api_token_allowed_services", token.AllowedServices)
	c.Set("api_token_openai_service", token.OpenAIService)
	c.Set("api_token_allowed_models", token.AllowedModels)
	c.Set("api_token_allowed_providers", token.AllowedProviders)
	if token.RateLimitRPM > 0 {
		c.Set("api_token_rate_limit_rule", config.RateLimitRule{
			RequestsPerMinute: token.RateLimitRPM,
			Burst:             token.RateLimitBurst,
		})
	}
}

func extractBearerToken(header string) string {
	header = strings.TrimSpace(header)
	if header == "" || !strings.HasPrefix(strings.ToLower(header), "bearer ") {
		return ""
	}
	return strings.TrimSpace(header[7:])
}

// SessionMiddleware rejects requests without a valid browser session or API token.
// Sets "session_user" and "auth_source" in the Gin context on success.
func SessionMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		user, source, ok := authenticateRequest(c)
		if !ok || user == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
			return
		}
		if source == "session" {
			if ip := clientIPv4(c.ClientIP()); ip != "" {
				c.Set("session_client_ip", ip)
			}
		}
		c.Next()
	}
}

// AdminMiddleware requires a browser session and admin role.
// API tokens are intentionally rejected here so admin writes remain tied to
// the CSRF-protected Web UI session path.
func AdminMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		user, ok := adminUserFromSession(c)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
			return
		}

		if !isAdminUser(user) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "admin access required"})
			return
		}
		c.Next()
	}
}

func MetricsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		expected := strings.TrimSpace(os.Getenv("APIG0_METRICS_TOKEN"))
		if expected != "" {
			raw := extractBearerToken(c.GetHeader("Authorization"))
			if raw == "" {
				raw = strings.TrimSpace(c.GetHeader("X-API-Key"))
			}
			if constantTimeStringEqual(raw, expected) {
				c.Set("auth_source", "metrics_token")
				c.Next()
				return
			}
		}

		if user, ok := adminUserFromSession(c); ok && isAdminUser(user) {
			c.Next()
			return
		}
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "metrics authentication required"})
	}
}

func adminUserFromSession(c *gin.Context) (string, bool) {
	tok, err := c.Cookie("apig0_session")
	if err != nil || tok == "" {
		return "", false
	}
	user, ok := ValidateSession(tok)
	if !ok || user == "" {
		return "", false
	}
	c.Set("session_user", user)
	c.Set("auth_source", "session")
	return user, true
}

func constantTimeStringEqual(a, b string) bool {
	if a == "" || b == "" || len(a) != len(b) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

func isAdminUser(user string) bool {
	store := config.GetUserStore()
	return store != nil && store.GetRole(user) == "admin"
}

// IsSecure returns true when the gateway should set Secure cookies.
// Looks at APIG0_SECURE env var ("true"/"false"). Defaults to false
// for local HTTP; main sets APIG0_SECURE=true when TLS is active.
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

func clientIPv4(raw string) string {
	ip := net.ParseIP(strings.TrimSpace(raw))
	if ip == nil {
		return ""
	}
	if v4 := ip.To4(); v4 != nil {
		return v4.String()
	}
	return ip.String()
}
