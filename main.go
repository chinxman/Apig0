// This entry point sets up the API gateway.
// URL namespace:
//   GET  /healthz                – health-check
//   GET  /dashboard              – live monitoring dashboard
//   GET  /api/admin/events       – SSE stream of request events
//   GET  /api/admin/stats        – JSON metrics snapshot
//   ANY  /*servicePath           – proxy to downstream services
package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
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

	// Load rate limits from file
	config.LoadRateLimits()

	// Unified WebUI — login overlay gates access, role determines visible panels.
	// Admin data still requires AdminMiddleware on /api/admin/*.
	r.GET("/", func(c *gin.Context) {
		c.File("webui.html")
	})
	r.GET("/dashboard", func(c *gin.Context) {
		c.Redirect(301, "/")
	})
	r.GET("/portal", func(c *gin.Context) {
		c.Redirect(301, "/")
	})

	// Auth endpoints — no session required, but rate-limited to prevent brute force
	// CSRF exempt: these are pre-authentication or use non-cookie credentials.
	r.POST("/auth/login", middleware.RateLimit(), auth.LoginHandler)
	r.POST("/auth/verify", middleware.RateLimit(), auth.VerifyHandler)
	r.POST("/auth/logout", auth.LogoutHandler)

	// Admin endpoints — session + admin role + CSRF required
	admin := r.Group("/api/admin")
	admin.Use(auth.AdminMiddleware())
	admin.Use(middleware.CSRF())
	admin.GET("/events", mon.SSEHandler())
	admin.GET("/stats", mon.StatsHandler())
	admin.GET("/users", auth.ListUsersHandler)
	admin.POST("/users", auth.CreateUserHandler)
	admin.DELETE("/users/:user", auth.DeleteUserHandler)
	admin.POST("/users/:user/reset", auth.ResetTOTPHandler)
	admin.GET("/settings/ratelimits", auth.GetRateLimitsHandler)
	admin.POST("/settings/ratelimits", auth.SaveRateLimitsHandler)

	// User info endpoint — session + rate limit required
	r.GET("/api/user/info", auth.SessionMiddleware(), middleware.RateLimit(), func(c *gin.Context) {
		user, _ := c.Get("session_user")
		role := config.GetUserStore().GetRole(user.(string))
		c.JSON(200, gin.H{"user": user, "role": role})
	})

	// Health-check
	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	// Catch-all proxy — session + rate limit + CSRF required
	r.NoRoute(auth.SessionMiddleware(), middleware.CSRF(), middleware.RateLimit(), func(c *gin.Context) {
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

	port := os.Getenv("APIG0_PORT")
	if port == "" {
		port = "8080"
	}

	tlsCfg := config.LoadTLSConfig()

	if tlsCfg.Mode != config.TLSOff {
		// TLS is active — auto-enable secure cookies
		os.Setenv("APIG0_SECURE", "true")

		httpsAddr := ":" + port
		log.Printf("Apig0 gateway running on %s (HTTPS)", httpsAddr)
		log.Printf("WebUI: https://0.0.0.0:%s/", port)
		log.Printf("TLS cert: %s", tlsCfg.CertFile)

		// Optional: redirect HTTP → HTTPS on port 8080 if HTTPS is on a different port
		if port != "8080" {
			go func() {
				redirect := http.NewServeMux()
				redirect.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
					target := fmt.Sprintf("https://%s:%s%s", r.Host, port, r.URL.RequestURI())
					http.Redirect(w, r, target, http.StatusMovedPermanently)
				})
				log.Println("HTTP redirect on :8080 → HTTPS")
				http.ListenAndServe(":8080", redirect)
			}()
		}

		if err := r.RunTLS(httpsAddr, tlsCfg.CertFile, tlsCfg.KeyFile); err != nil {
			log.Fatalf("[tls] failed to start: %v", err)
		}
	} else {
		addr := ":" + port
		log.Printf("Apig0 gateway running on %s (HTTP)", addr)
		log.Printf("WebUI: http://0.0.0.0:%s/", port)
		r.Run(addr)
	}
}
