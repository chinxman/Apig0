package proxy

import (
	"log"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
)

// NewReverseProxy creates a reverse proxy to the given internal backend.
// routePrefix is the public path exposed by the gateway (e.g., /users)
func NewReverseProxy(routePrefix string, backendURL string) gin.HandlerFunc {

	u, err := url.Parse(backendURL)
	if err != nil {
		log.Fatal(err)
	}

	proxy := httputil.NewSingleHostReverseProxy(u)

	return func(c *gin.Context) {
		// strip the route prefix so backend sees just / or /rest-of-path
		c.Request.URL.Path = strings.TrimPrefix(c.Request.URL.Path, routePrefix)
		if c.Request.URL.Path == "" {
			c.Request.URL.Path = "/" // ensure path is at least /
		}

		// preserve client IP
		c.Request.Header.Set("X-Forwarded-For", c.ClientIP())

		proxy.ServeHTTP(c.Writer, c.Request)
	}
}
