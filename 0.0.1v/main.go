// This entry point sets up the API gateway.
// URL namespace:
//   GET  /healthz                – health‑check
//   ANY  /*servicePath           – proxy to downstream services (users, products, orders)
package main // edited placeholder

import (
	"log"
	"strings"

	"apig0/auth"
	"apig0/proxy"
	"apig0/middleware"

	"github.com/gin-gonic/gin"
)

func main() {

	auth.PrintQRIfEnabled("devin")

	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()
	r.SetTrustedProxies([]string{"127.0.0.1", "192.168.12.0/24"})

	// CORS middleware
	r.Use(middleware.Cors())

	// apply TOTP middleware globally
	r.Use(auth.Middleware())

	// service to backend mapping
	// Add new services here: key is the public path segment (e.g., /users)
	// and the value is the reverse proxy handler pointing to the backend service
	serviceProxies := map[string]gin.HandlerFunc{
		"users":    proxy.NewReverseProxy("", "http://192.168.12.11:3001"),
		"products": proxy.NewReverseProxy("", "http://192.168.12.11:3002"),
		"orders":   proxy.NewReverseProxy("", "http://192.168.12.11:3003"),
	}

	// health‑check endpoint
	r.GET("/healthz", func(c *gin.Context) {
		log.Printf("[healthz] Request from %s", c.ClientIP())
		c.JSON(200, gin.H{"status": "ok"})
	})

	// catch-all route for services (must come after /healthz to avoid conflict)
	// This handles: /users/*, /products/*, /orders/*
	r.NoRoute(func(c *gin.Context) {
		path := strings.TrimPrefix(c.Request.URL.Path, "/")
		if path == "" {
			c.Next()
			return
		}
		parts := strings.SplitN(path, "/", 2)
		svc := parts[0]

		// Only handle if it's a known service
		if handler, ok := serviceProxies[svc]; ok {
			handler(c)
			return
		}

		// Not a known service, return 404
		log.Printf("[proxy] Service not found: %s", svc)
		c.JSON(404, gin.H{"error": "service not found"})
	})

	log.Println("Apig0 gateway running on :8080")
	r.Run(":8080")
}
