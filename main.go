// This entry point sets up the API gateway.
// URL namespace:
//
//	GET  /healthz                – health-check
//	GET  /dashboard              – live monitoring dashboard
//	GET  /api/admin/events       – SSE stream of request events
//	GET  /api/admin/stats        – JSON metrics snapshot
//	ANY  /*servicePath           – proxy to downstream services
package main

import (
	"fmt"
	"log"
	"net"
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
	config.LoadSetupBootstrap()
	config.LoadAppConfig()
	if err := config.ReloadRuntime(nil, ""); err != nil {
		log.Printf("[setup] runtime init warning: %v", err)
	}

	if users := strings.Split(strings.TrimSpace(os.Getenv("APIG0_USERS")), ","); len(users) > 0 {
		user := strings.TrimSpace(users[0])
		if user != "" {
			auth.PrintQRIfEnabled(user)
		}
	}

	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()
	r.SetTrustedProxies([]string{"127.0.0.1", "192.168.12.0/24"})

	// Create monitor and register known services
	mon := middleware.NewMonitor()
	for name, backend := range config.GetServices() {
		mon.RegisterService(name, backend)
	}

	// Global middleware: CORS → Monitor
	// Auth is enforced per-route: SessionMiddleware on proxy + admin endpoints.
	r.Use(middleware.Cors())
	r.Use(mon.Middleware())

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
	r.GET("/api/setup/status", auth.SetupStatusHandler)
	r.POST("/api/setup/reset", auth.ResetSetupHandler)
	r.POST("/api/setup/complete", auth.CompleteSetupHandler)
	r.POST("/api/setup/bootstrap-admin", middleware.RateLimit(), auth.BootstrapAdminHandler)
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
	admin.PUT("/users/:user/access", auth.UpdateUserAccessHandler)
	admin.GET("/settings/ratelimits", auth.GetRateLimitsHandler)
	admin.POST("/settings/ratelimits", auth.SaveRateLimitsHandler)
	admin.POST("/settings/storage", auth.UpgradeStorageHandler)

	// User info endpoint — session + rate limit required
	r.GET("/api/user/info", auth.SessionMiddleware(), middleware.RateLimit(), func(c *gin.Context) {
		user, _ := c.Get("session_user")
		role := config.GetUserStore().GetRole(user.(string))
		services := config.ListServiceNames()
		allowed := services
		if role != "admin" {
			explicit := config.GetUserStore().GetAllowedServices(user.(string))
			if config.GetUserStore().HasConfiguredServiceAccess(user.(string)) {
				allowed = explicit
			}
		}
		c.JSON(200, gin.H{
			"user":                      user,
			"role":                      role,
			"service_access_configured": role == "admin" || config.GetUserStore().HasConfiguredServiceAccess(user.(string)),
			"available_services":        allowed,
			"service_catalog":           services,
		})
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

		if serviceCfg, ok := config.LookupService(svc); ok {
			userVal, _ := c.Get("session_user")
			username, _ := userVal.(string)
			if username == "" || !config.GetUserStore().CanAccessService(username, svc) {
				c.JSON(http.StatusForbidden, gin.H{"error": "service access denied"})
				return
			}
			mon.RegisterService(serviceCfg.Name, serviceCfg.BaseURL)
			proxy.NewReverseProxy(serviceCfg)(c)
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
		webUIHost := advertisedHost()
		log.Printf("Apig0 gateway running on %s (HTTPS)", httpsAddr)
		log.Printf("WebUI: https://%s:%s/", webUIHost, port)
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
		webUIHost := advertisedHost()
		log.Printf("Apig0 gateway running on %s (HTTP)", addr)
		log.Printf("WebUI: http://%s:%s/", webUIHost, port)
		r.Run(addr)
	}
}

func advertisedHost() string {
	if host := strings.TrimSpace(os.Getenv("APIG0_ADVERTISE_HOST")); host != "" {
		return host
	}

	ifaces, err := net.Interfaces()
	if err != nil {
		return "127.0.0.1"
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip == nil || ip.IsLoopback() {
				continue
			}
			if ipv4 := ip.To4(); ipv4 != nil {
				return ipv4.String()
			}
		}
	}
	return "127.0.0.1"
}
