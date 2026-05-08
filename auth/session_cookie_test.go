package auth

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestSetSessionCookieDefaultsToSecure(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("APIG0_SECURE", "")
	t.Setenv("APIG0_INSECURE_COOKIES", "")
	t.Setenv("APIG0_SESSION_TTL", "2h")

	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)

	SetSessionCookie(ctx, "token")

	header := w.Header().Get("Set-Cookie")
	for _, want := range []string{"Secure", "HttpOnly", "SameSite=Strict", "Max-Age=7200"} {
		if !strings.Contains(header, want) {
			t.Fatalf("expected %q in Set-Cookie header %q", want, header)
		}
	}
}

func TestSetSessionCookieAllowsExplicitInsecureDevMode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("APIG0_INSECURE_COOKIES", "true")
	t.Setenv("APIG0_SECURE", "")

	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)

	SetSessionCookie(ctx, "token")

	header := w.Header().Get("Set-Cookie")
	if strings.Contains(header, "Secure") {
		t.Fatalf("expected insecure dev cookie without Secure flag, got %q", header)
	}
	if !strings.Contains(header, "HttpOnly") {
		t.Fatalf("expected HttpOnly in Set-Cookie header %q", header)
	}
}

func TestClearSessionCookieUsesSameCookiePolicy(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("APIG0_INSECURE_COOKIES", "")
	t.Setenv("APIG0_SECURE", "")

	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)

	ClearSessionCookie(ctx)

	header := w.Header().Get("Set-Cookie")
	for _, want := range []string{"Secure", "HttpOnly", "SameSite=Strict", "Max-Age=0"} {
		if !strings.Contains(header, want) {
			t.Fatalf("expected %q in Set-Cookie header %q", want, header)
		}
	}
}
