// This entry point sets up the API gateway.
// URL namespace:
//   GET  /healthz                – health-check
//   GET  /dashboard              – live monitoring dashboard
//   GET  /api/admin/events       – SSE stream of request events
//   GET  /api/admin/stats        – JSON metrics snapshot
//   ANY  /*servicePath           – proxy to downstream services
package main

import (
	"log"
	"strings"

	"apig0/auth"
	"apig0/config"
	"apig0/middleware"
	"apig0/proxy"

	"github.com/gin-gonic/gin"
)

func main() {
	config.LoadAppConfig()
	config.InitSecrets()

	auth.PrintQRIfEnabled("devin")

	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()
	r.SetTrustedProxies([]string{"127.0.0.1", "192.168.12.0/24"})

	// Service to backend mapping
	services := map[string]string{
		"users":    "http://192.168.12.11:3001",
		"products": "http://192.168.12.11:3002",
		"orders":   "http://192.168.12.11:3003",
	}

	// Create monitor and register known services
	mon := middleware.NewMonitor()
	for name, backend := range services {
		mon.RegisterService(name, backend)
	}

	// Global middleware: CORS → Monitor
	// Auth is enforced per-route: SessionMiddleware on proxy + admin endpoints.
	r.Use(middleware.Cors())
	r.Use(mon.Middleware())

	// Build proxy handlers
	serviceProxies := make(map[string]gin.HandlerFunc, len(services))
	for name, backend := range services {
		serviceProxies[name] = proxy.NewReverseProxy(backend)
	}

	// Dashboard (public — login overlay gates access in the browser)
	r.GET("/dashboard", func(c *gin.Context) {
		c.File("dashboard.html")
	})

	// Auth endpoints — no TOTP, no session required
	r.POST("/auth/login", auth.LoginHandler)
	r.POST("/auth/verify", auth.VerifyHandler)
	r.POST("/auth/logout", auth.LogoutHandler)

	// Admin endpoints — session required
	admin := r.Group("/api/admin")
	admin.Use(auth.SessionMiddleware())
	admin.GET("/events", mon.SSEHandler())
	admin.GET("/stats", mon.StatsHandler())

	// Health-check
	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	// Catch-all proxy — session required
	r.NoRoute(auth.SessionMiddleware(), func(c *gin.Context) {
		path := strings.TrimPrefix(c.Request.URL.Path, "/")
		if path == "" {
			c.Next()
			return
		}
		parts := strings.SplitN(path, "/", 2)
		svc := parts[0]

		if handler, ok := serviceProxies[svc]; ok {
			handler(c)
			return
		}

		c.JSON(404, gin.H{"error": "service not found"})
	})

	log.Println("Apig0 gateway running on :8080")
	log.Println("Dashboard: http://0.0.0.0:8080/dashboard")
	r.Run(":8080")
}
