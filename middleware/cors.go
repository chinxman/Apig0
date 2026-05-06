package middleware

import (
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
)

// Cors returns a Gin middleware that sets CORS headers and handles OPTIONS pre-flight requests.
// By default no cross-origin browser access is allowed; same-origin browser
// traffic does not need CORS headers. Set APIG0_CORS_ORIGINS to a comma-separated
// allowlist when a trusted browser origin must call the gateway cross-origin.
func Cors() gin.HandlerFunc {
	allowedOrigins := parseAllowedOrigins(os.Getenv("APIG0_CORS_ORIGINS"))
	allowWildcard := allowedOrigins["*"] && strings.EqualFold(os.Getenv("APIG0_CORS_ALLOW_WILDCARD"), "true")

	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if allowedOrigin(origin, allowedOrigins) {
			c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
			c.Writer.Header().Set("Vary", "Origin")
		} else if allowWildcard && origin != "" {
			c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		} else if origin != "" && c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusForbidden)
			return
		}
		if c.Writer.Header().Get("Access-Control-Allow-Origin") != "" {
			c.Writer.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,PATCH,DELETE,OPTIONS")
			c.Writer.Header().Set("Access-Control-Allow-Headers", "Authorization,Content-Type,X-API-Key,X-CSRF-Token")
			c.Writer.Header().Set("Access-Control-Max-Age", "600")
		}

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

func parseAllowedOrigins(raw string) map[string]bool {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return map[string]bool{}
	}
	allowed := map[string]bool{}
	for _, part := range strings.Split(raw, ",") {
		if origin := strings.TrimSpace(part); origin != "" {
			allowed[origin] = true
		}
	}
	if len(allowed) == 0 {
		allowed["*"] = true
	}
	return allowed
}

func allowedOrigin(origin string, allowed map[string]bool) bool {
	origin = strings.TrimSpace(origin)
	if origin == "" {
		return false
	}
	return allowed[origin]
}
