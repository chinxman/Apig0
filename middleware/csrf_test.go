package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestCSRFCookieDefaultsToSecureAndReadableByJS(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("APIG0_SECURE", "")
	t.Setenv("APIG0_INSECURE_COOKIES", "")

	router := gin.New()
	router.Use(CSRF())
	router.GET("/", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	header := w.Header().Get("Set-Cookie")
	for _, want := range []string{"apig0_csrf=", "Secure", "SameSite=Strict"} {
		if !strings.Contains(header, want) {
			t.Fatalf("expected %q in Set-Cookie header %q", want, header)
		}
	}
	if strings.Contains(header, "HttpOnly") {
		t.Fatalf("expected JS-readable CSRF cookie without HttpOnly, got %q", header)
	}
}

func TestCSRFCookieAllowsExplicitInsecureDevMode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("APIG0_INSECURE_COOKIES", "true")
	t.Setenv("APIG0_SECURE", "")

	router := gin.New()
	router.Use(CSRF())
	router.GET("/", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	header := w.Header().Get("Set-Cookie")
	if strings.Contains(header, "Secure") {
		t.Fatalf("expected insecure dev CSRF cookie without Secure flag, got %q", header)
	}
	if strings.Contains(header, "HttpOnly") {
		t.Fatalf("expected JS-readable CSRF cookie without HttpOnly, got %q", header)
	}
}
