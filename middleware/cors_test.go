package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestCorsPreflightAllowsConfiguredAPIHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("APIG0_CORS_ORIGINS", "https://client.example")

	router := gin.New()
	router.Use(Cors())
	router.OPTIONS("/resource", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodOptions, "/resource", nil)
	req.Header.Set("Origin", "https://client.example")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", w.Code)
	}
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "https://client.example" {
		t.Fatalf("unexpected allow-origin header: %q", got)
	}
	allowedHeaders := w.Header().Get("Access-Control-Allow-Headers")
	for _, header := range []string{"Authorization", "X-API-Key", "X-CSRF-Token"} {
		if !strings.Contains(allowedHeaders, header) {
			t.Fatalf("missing CORS header %q in %q", header, allowedHeaders)
		}
	}
}

func TestCorsDefaultRejectsCrossOriginPreflight(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("APIG0_CORS_ORIGINS", "")

	router := gin.New()
	router.Use(Cors())
	router.OPTIONS("/resource", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodOptions, "/resource", nil)
	req.Header.Set("Origin", "https://client.example")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("unexpected allow-origin header: %q", got)
	}
}

func TestCorsRestrictsConfiguredOrigins(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("APIG0_CORS_ORIGINS", "https://admin.example")

	router := gin.New()
	router.Use(Cors())
	router.GET("/resource", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	allowedReq := httptest.NewRequest(http.MethodGet, "/resource", nil)
	allowedReq.Header.Set("Origin", "https://admin.example")
	allowed := httptest.NewRecorder()
	router.ServeHTTP(allowed, allowedReq)

	if got := allowed.Header().Get("Access-Control-Allow-Origin"); got != "https://admin.example" {
		t.Fatalf("unexpected allowed origin: %q", got)
	}

	deniedReq := httptest.NewRequest(http.MethodGet, "/resource", nil)
	deniedReq.Header.Set("Origin", "https://evil.example")
	denied := httptest.NewRecorder()
	router.ServeHTTP(denied, deniedReq)

	if got := denied.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("unexpected CORS header for denied origin: %q", got)
	}
}
