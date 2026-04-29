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
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"slices"
	"strings"

	"apig0/auth"
	"apig0/cli"
	"apig0/config"
	"apig0/middleware"
	"apig0/proxy"

	"github.com/gin-gonic/gin"
)

func main() {
	if cli.ShouldSilenceBootstrapLogs(os.Args[1:]) {
		log.SetOutput(io.Discard)
	}
	config.LoadSetupBootstrap()
	config.LoadAppConfig()
	if err := config.ReloadRuntime(nil, ""); err != nil {
		log.Printf("[setup] runtime init warning: %v", err)
	}
	if handled, exitCode := cli.Run(os.Args[1:]); handled {
		os.Exit(exitCode)
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
	r.Static("/static", "./static")

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
	admin.GET("/users/:user/policies", auth.GetUserPoliciesHandler)
	admin.PUT("/users/:user/policies", auth.UpdateUserPoliciesHandler)
	admin.GET("/tokens", auth.ListTokensHandler)
	admin.POST("/tokens", auth.CreateTokenHandler)
	admin.POST("/tokens/:id/revoke", auth.RevokeTokenHandler)
	admin.GET("/audit", auth.ListAuditHandler)
	admin.GET("/services", auth.ListServicesHandler)
	admin.POST("/services", auth.CreateServiceHandler)
	admin.PUT("/services/:name", auth.UpdateServiceHandler)
	admin.DELETE("/services/:name", auth.DeleteServiceHandler)
	admin.POST("/services/:name/test-auth", auth.TestServiceAuthHandler)
	admin.GET("/settings/ratelimits", auth.GetRateLimitsHandler)
	admin.POST("/settings/ratelimits", auth.SaveRateLimitsHandler)
	admin.POST("/settings/storage", auth.UpgradeStorageHandler)
	admin.POST("/setup/reset", auth.ResetSetupHandler)
	r.GET("/metrics", middleware.PrometheusHandler(mon))

	// User info endpoint — session + rate limit required
	r.GET("/api/user/info", auth.SessionMiddleware(), middleware.RateLimit(), func(c *gin.Context) {
		user, _ := c.Get("session_user")
		username := user.(string)
		role := config.GetUserStore().GetRole(username)
		services := config.ListServiceNames()
		allowed := services
		if role != "admin" {
			explicit := config.GetUserStore().GetAllowedServices(username)
			if config.GetUserStore().HasConfiguredServiceAccess(username) {
				allowed = explicit
			}
		}
		if scoped, ok := c.Get("api_token_allowed_services"); ok {
			if tokenAllowed, ok := scoped.([]string); ok && len(tokenAllowed) > 0 {
				allowed = tokenAllowed
			}
		}
		assignedToken, hasAssignedToken := config.GetLatestActiveTokenForUser(username)
		assignedBackendLabel := ""
		if hasAssignedToken && assignedToken.OpenAIService != "" {
			if backend, ok := config.GetServiceConfig(assignedToken.OpenAIService); ok {
				switch {
				case strings.TrimSpace(backend.Provider) != "" && backend.Provider != backend.Name:
					assignedBackendLabel = backend.Provider + " • " + backend.Name
				case strings.TrimSpace(backend.Provider) != "":
					assignedBackendLabel = backend.Provider
				default:
					assignedBackendLabel = backend.Name
				}
			}
		}
		c.JSON(200, gin.H{
			"user":                       user,
			"role":                       role,
			"auth_source":                c.GetString("auth_source"),
			"service_access_configured":  role == "admin" || config.GetUserStore().HasConfiguredServiceAccess(username),
			"available_services":         allowed,
			"service_catalog":            services,
			"node_mode":                  config.GetRuntimeStatus().NodeMode,
			"has_assigned_token":         hasAssignedToken,
			"assigned_token_type":        assignedToken.KeyType,
			"assigned_token_prefix":      assignedToken.TokenPrefix,
			"assigned_openai_service":    assignedToken.OpenAIService,
			"assigned_backend_label":     assignedBackendLabel,
			"assigned_allowed_models":    assignedToken.AllowedModels,
			"assigned_allowed_providers": assignedToken.AllowedProviders,
		})
	})
	r.GET("/api/user/pending-keys", auth.SessionMiddleware(), middleware.RateLimit(), auth.ListPendingTokenDeliveriesHandler)
	r.POST("/api/user/pending-keys/:id/claim", auth.SessionMiddleware(), middleware.CSRF(), middleware.RateLimit(), auth.ClaimPendingTokenDeliveryHandler)

	// OpenAI-compatible AI proxy — token auth + rate limit, no CSRF so SDKs can call it directly.
	r.Any("/openai/v1", auth.SessionMiddleware(), middleware.RateLimit(), proxy.NewOpenAICompatibleProxy())
	r.Any("/openai/v1/*openaiPath", auth.SessionMiddleware(), middleware.RateLimit(), proxy.NewOpenAICompatibleProxy())

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
		routePath := "/"
		if len(parts) == 2 && strings.TrimSpace(parts[1]) != "" {
			routePath = "/" + parts[1]
		}

		if serviceCfg, ok := config.LookupService(svc); ok {
			userVal, _ := c.Get("session_user")
			username, _ := userVal.(string)
			authSource := c.GetString("auth_source")
			if username == "" {
				config.RecordAuditEvent(config.AuditEvent{
					User:       username,
					AuthSource: authSource,
					Service:    svc,
					Method:     c.Request.Method,
					Path:       routePath,
					Decision:   "deny",
					Reason:     "authentication required",
					ClientIP:   c.ClientIP(),
					Status:     http.StatusUnauthorized,
					Upstream:   serviceCfg.BaseURL,
				})
				c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
				return
			}
			if scoped, ok := c.Get("api_token_allowed_services"); ok {
				if allowed, ok := scoped.([]string); ok && len(allowed) > 0 && !slices.Contains(allowed, svc) {
					config.RecordAuditEvent(config.AuditEvent{
						User:       username,
						AuthSource: authSource,
						Service:    svc,
						Method:     c.Request.Method,
						Path:       routePath,
						Decision:   "deny",
						Reason:     "token scope denied",
						PolicyID:   "",
						TokenID:    c.GetString("api_token_id"),
						ClientIP:   c.ClientIP(),
						Status:     http.StatusForbidden,
						Upstream:   serviceCfg.BaseURL,
					})
					c.JSON(http.StatusForbidden, gin.H{"error": "token scope denied"})
					return
				}
			}
			decision := config.EvaluateRouteAccess(username, svc, c.Request.Method, routePath, authSource)
			if !decision.Allowed {
				config.RecordAuditEvent(config.AuditEvent{
					User:       username,
					AuthSource: authSource,
					Service:    svc,
					Method:     c.Request.Method,
					Path:       routePath,
					Decision:   "deny",
					Reason:     decision.Reason,
					PolicyID:   decision.PolicyID,
					TokenID:    c.GetString("api_token_id"),
					ClientIP:   c.ClientIP(),
					Status:     http.StatusForbidden,
					Upstream:   serviceCfg.BaseURL,
				})
				c.JSON(http.StatusForbidden, gin.H{"error": decision.Reason})
				return
			}
			mon.RegisterService(serviceCfg.Name, serviceCfg.BaseURL)
			proxy.NewReverseProxy(serviceCfg)(c)
			config.RecordAuditEvent(config.AuditEvent{
				User:       username,
				AuthSource: authSource,
				Service:    svc,
				Method:     c.Request.Method,
				Path:       routePath,
				Decision:   "allow",
				Reason:     decision.Reason,
				PolicyID:   decision.PolicyID,
				TokenID:    c.GetString("api_token_id"),
				ClientIP:   c.ClientIP(),
				Status:     c.Writer.Status(),
				Upstream:   serviceCfg.BaseURL,
			})
			return
		}

		config.RecordAuditEvent(config.AuditEvent{
			User:       c.GetString("session_user"),
			AuthSource: c.GetString("auth_source"),
			Service:    svc,
			Method:     c.Request.Method,
			Path:       routePath,
			Decision:   "deny",
			Reason:     "service not found",
			TokenID:    c.GetString("api_token_id"),
			ClientIP:   c.ClientIP(),
			Status:     http.StatusNotFound,
		})
		c.JSON(404, gin.H{"error": "service not found"})
	})

	port := os.Getenv("APIG0_PORT")
	if port == "" {
		port = "8989"
	}

	tlsCfg := config.LoadTLSConfig()

	if tlsCfg.Mode != config.TLSOff {
		// TLS is active — auto-enable secure cookies
		os.Setenv("APIG0_SECURE", "true")

		httpsAddr := ":" + port
		webUIHost := advertisedHost()
		logStartupSummary("https", webUIHost, port, tlsCfg)

		if err := r.RunTLS(httpsAddr, tlsCfg.CertFile, tlsCfg.KeyFile); err != nil {
			log.Fatalf("[tls] failed to start: %v", err)
		}
	} else {
		addr := ":" + port
		webUIHost := advertisedHost()
		_ = addr
		logStartupSummary("http", webUIHost, port, tlsCfg)
		r.Run(addr)
	}
}

func logStartupSummary(scheme, host, port string, tlsCfg config.TLSConfig) {
	status := config.GetRuntimeStatus()
	log.Printf("[startup] Apig0 ready: %s://%s:%s/ (setup=%s, node=%s)", scheme, host, port, status.SetupMode, status.NodeMode)
	if scheme == "https" && tlsCfg.Mode == config.TLSAuto {
		if status.SetupMode == string(config.SetupModeTemporary) {
			log.Printf("[startup] TLS: temporary auto-cert active")
			return
		}
		log.Printf("[startup] TLS: auto-cert active (%s)", tlsCfg.CertFile)
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
