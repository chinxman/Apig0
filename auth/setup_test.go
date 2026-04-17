package auth

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"apig0/config"

	"github.com/gin-gonic/gin"
)

func TestBootstrapAdminHandlerCreatesFirstAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)

	config.ActivateTemporarySetup(config.SetupConfig{
		Mode:           config.SetupModeTemporary,
		Port:           "8080",
		UserVault:      config.UserVaultSettings{Type: "file", FilePath: filepath.Join(t.TempDir(), "totp-secrets.json")},
		ServiceSecrets: config.ServiceSecretConfig{Mode: config.ServiceSecretMemory},
	})
	config.InitUserStore(filepath.Join(t.TempDir(), "users.json"))
	t.Cleanup(func() {
		config.InitUserStore(filepath.Join(t.TempDir(), "users-cleanup.json"))
	})

	t.Setenv("VAULT_TYPE", "file")
	t.Setenv("VAULT_FILE_PATH", filepath.Join(t.TempDir(), "totp-secrets.json"))
	config.LoadVaultSecrets()

	body, _ := json.Marshal(map[string]string{
		"username": "admin",
		"password": "secret123",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/setup/bootstrap-admin", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	ctx, _ := gin.CreateTestContext(w)
	ctx.Request = req
	BootstrapAdminHandler(ctx)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if !config.GetUserStore().Exists("admin") {
		t.Fatal("expected bootstrapped admin to exist")
	}
	if config.GetUserStore().GetRole("admin") != "admin" {
		t.Fatal("expected bootstrapped user to be admin")
	}
}

func TestBootstrapAdminHandlerRejectsWhenAdminExists(t *testing.T) {
	gin.SetMode(gin.TestMode)

	config.ActivateTemporarySetup(config.SetupConfig{
		Mode:           config.SetupModeTemporary,
		Port:           "8080",
		UserVault:      config.UserVaultSettings{Type: "file", FilePath: filepath.Join(t.TempDir(), "totp-secrets.json")},
		ServiceSecrets: config.ServiceSecretConfig{Mode: config.ServiceSecretMemory},
	})
	t.Setenv("VAULT_TYPE", "file")
	t.Setenv("VAULT_FILE_PATH", filepath.Join(t.TempDir(), "totp-secrets.json"))
	config.LoadVaultSecrets()
	config.InitUserStore(filepath.Join(t.TempDir(), "users.json"))
	if err := config.GetUserStore().Create("root", "secret123", "admin", nil, false); err != nil {
		t.Fatalf("seed admin: %v", err)
	}
	t.Cleanup(func() {
		config.InitUserStore(filepath.Join(t.TempDir(), "users-cleanup.json"))
	})

	body, _ := json.Marshal(map[string]string{
		"username": "admin2",
		"password": "secret123",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/setup/bootstrap-admin", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	ctx, _ := gin.CreateTestContext(w)
	ctx.Request = req
	BootstrapAdminHandler(ctx)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", w.Code, w.Body.String())
	}
}
