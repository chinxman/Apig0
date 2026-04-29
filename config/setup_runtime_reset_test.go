package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResetSetupStateRemovesPersistentFilesAndReturnsToSetup(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}

	tmp := t.TempDir()
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir temp dir: %v", err)
	}
	defer os.Chdir(wd)

	setupFilePath = filepath.Join(tmp, "apig0-setup.json")
	t.Cleanup(func() {
		setupFilePath = "apig0-setup.json"
	})

	cfg := SetupConfig{
		Mode:           SetupModePersistent,
		Port:           "8989",
		UsersPath:      filepath.Join(tmp, "users.json"),
		ServicesPath:   filepath.Join(tmp, "services.json"),
		RateLimitsPath: filepath.Join(tmp, "ratelimits.json"),
		UserVault:      UserVaultSettings{Type: "file", FilePath: filepath.Join(tmp, "totp-secrets.json")},
		ServiceSecrets: ServiceSecretConfig{Mode: ServiceSecretFile, FilePath: filepath.Join(tmp, "service-secrets.json")},
	}
	if err := SavePersistentSetup(cfg); err != nil {
		t.Fatalf("save setup: %v", err)
	}

	files := []string{
		cfg.UsersPath,
		cfg.ServicesPath,
		cfg.RateLimitsPath,
		cfg.UserVault.FilePath,
		cfg.ServiceSecrets.FilePath,
	}
	for _, path := range files {
		if err := os.WriteFile(path, []byte("{}"), 0600); err != nil {
			t.Fatalf("seed %s: %v", path, err)
		}
	}

	if err := ResetSetupState(); err != nil {
		t.Fatalf("reset setup: %v", err)
	}

	for _, path := range append(files, setupFilePath) {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("expected %s to be removed, got err=%v", path, err)
		}
	}

	status := GetRuntimeStatus()
	if !status.SetupRequired {
		t.Fatal("expected setup to be required after reset")
	}
	if status.PersistentConfigured {
		t.Fatal("expected persistent setup to be cleared")
	}
}

func TestBootstrapUsersSkipsWhileSetupPending(t *testing.T) {
	setupFilePath = filepath.Join(t.TempDir(), "apig0-setup.json")
	t.Cleanup(func() {
		setupFilePath = "apig0-setup.json"
	})

	activeSetup = defaultSetupConfig()
	setupConfigured = false

	store := &UserStore{
		cache:    map[string]*User{},
		filePath: filepath.Join(t.TempDir(), "users.json"),
	}
	globalUserStore = store
	t.Cleanup(func() {
		globalUserStore = nil
		UserPasswords = map[string]string{}
	})

	UserPasswords = map[string]string{"devin": "hash"}
	t.Setenv("APIG0_USERS", "devin")

	bootstrapUsers()

	if store.Exists("devin") {
		t.Fatal("expected bootstrap users to be skipped while setup is pending")
	}
}
