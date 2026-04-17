package proxy

import (
	"encoding/base64"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"

	"apig0/config"

	"github.com/gin-gonic/gin"
)

// NewReverseProxy creates a reverse proxy to the given backend service.
func NewReverseProxy(service config.ServiceConfig) gin.HandlerFunc {
	u, err := url.Parse(service.BaseURL)
	if err != nil {
		log.Fatal(err)
	}

	proxy := httputil.NewSingleHostReverseProxy(u)
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		applyServiceAuth(req, service)
	}

	return func(c *gin.Context) {
		c.Request.Header.Set("X-Forwarded-For", c.ClientIP())
		proxy.ServeHTTP(c.Writer, c.Request)
	}
}

func applyServiceAuth(req *http.Request, service config.ServiceConfig) {
	secret, hasSecret, err := config.GetServiceSecret(service.Name)
	if err != nil || !hasSecret {
		return
	}

	switch service.AuthType {
	case config.ServiceAuthBearer:
		req.Header.Set("Authorization", "Bearer "+secret)
	case config.ServiceAuthXAPIKey:
		header := service.HeaderName
		if header == "" {
			header = "X-API-Key"
		}
		req.Header.Set(header, secret)
	case config.ServiceAuthCustomHeader:
		header := service.HeaderName
		if header == "" {
			header = "Authorization"
		}
		req.Header.Set(header, secret)
	case config.ServiceAuthBasic:
		if service.BasicUsername == "" {
			return
		}
		token := base64.StdEncoding.EncodeToString([]byte(service.BasicUsername + ":" + secret))
		req.Header.Set("Authorization", "Basic "+token)
	}
}
