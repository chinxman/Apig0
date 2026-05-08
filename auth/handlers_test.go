package auth

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"apig0/config"

	"github.com/gin-gonic/gin"
	"github.com/pquerna/otp/totp"
)

func TestVerifyHandlerRecordsTOTPFailuresForLockout(t *testing.T) {
	setupAuthHandlerTest(t)

	challenge := NewChallenge("alice")
	for i := 0; i < maxFailedAttempts; i++ {
		w := runVerifyRequest(t, challenge, "000000")
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d: expected 401, got %d: %s", i+1, w.Code, w.Body.String())
		}
	}

	if !IsLockedOut("alice") {
		t.Fatal("expected invalid TOTP attempts to lock the account")
	}

	validCode, err := totp.GenerateCode(config.UserSecrets["alice"], time.Now())
	if err != nil {
		t.Fatalf("generate valid TOTP: %v", err)
	}
	w := runVerifyRequest(t, challenge, validCode)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 for locked account, got %d: %s", w.Code, w.Body.String())
	}
}

func TestVerifyHandlerClearsFailuresAfterSuccessfulMFA(t *testing.T) {
	setupAuthHandlerTest(t)

	RecordFailure("alice")
	RecordFailure("alice")
	challenge := NewChallenge("alice")
	validCode, err := totp.GenerateCode(config.UserSecrets["alice"], time.Now())
	if err != nil {
		t.Fatalf("generate valid TOTP: %v", err)
	}

	w := runVerifyRequest(t, challenge, validCode)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if hasLockoutEntry("alice") {
		t.Fatal("expected successful MFA to clear failed-attempt state")
	}
}

func setupAuthHandlerTest(t *testing.T) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	ResetSessionState()
	resetLockoutState()
	resetReplayState()

	dir := t.TempDir()
	config.ActivateTemporarySetup(config.SetupConfig{
		Mode:           config.SetupModeTemporary,
		Port:           "8989",
		UsersPath:      filepath.Join(dir, "users.json"),
		UserVault:      config.UserVaultSettings{Type: "file", FilePath: filepath.Join(dir, "totp-secrets.json")},
		ServiceSecrets: config.ServiceSecretConfig{Mode: config.ServiceSecretMemory},
	})
	config.InitUserStore(filepath.Join(dir, "users.json"))
	if err := config.GetUserStore().Create("alice", "correct horse battery staple", "user", nil, false); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	config.UserSecrets = map[string]string{"alice": "JBSWY3DPEHPK3PXP"}

	t.Cleanup(func() {
		ResetSessionState()
		resetLockoutState()
		resetReplayState()
		config.UserSecrets = map[string]string{}
	})
}

func runVerifyRequest(t *testing.T, challenge, code string) *httptest.ResponseRecorder {
	t.Helper()
	body, err := json.Marshal(map[string]string{
		"challenge": challenge,
		"code":      code,
	})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/auth/verify", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Request = req
	VerifyHandler(ctx)
	return w
}

func resetLockoutState() {
	lockoutMu.Lock()
	defer lockoutMu.Unlock()
	lockouts = map[string]*lockoutEntry{}
}

func hasLockoutEntry(username string) bool {
	lockoutMu.Lock()
	defer lockoutMu.Unlock()
	_, ok := lockouts[username]
	return ok
}

func resetReplayState() {
	replayMu.Lock()
	defer replayMu.Unlock()
	usedCodes = map[string]map[string]time.Time{}
}
