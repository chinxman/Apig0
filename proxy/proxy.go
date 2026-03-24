package proxy

import (
	"log"
	"net/http/httputil"
	"net/url"

	"github.com/gin-gonic/gin"
)

// NewReverseProxy creates a reverse proxy to the given internal backend.
func NewReverseProxy(backendURL string) gin.HandlerFunc {

	u, err := url.Parse(backendURL)
	if err != nil {
		log.Fatal(err)
	}

	proxy := httputil.NewSingleHostReverseProxy(u)

	return func(c *gin.Context) {
		// preserve client IP
		c.Request.Header.Set("X-Forwarded-For", c.ClientIP())

		proxy.ServeHTTP(c.Writer, c.Request)
	}
}
