package config

import (
	"path/filepath"
	"testing"
)

func TestFileVaultPersistsSecrets(t *testing.T) {
	v := NewFileVault(&VaultConfig{FilePath: filepath.Join(t.TempDir(), "totp-secrets.json")})

	if err := v.StoreSecret("totp", "alice", "SECRET1"); err != nil {
		t.Fatalf("store secret: %v", err)
	}

	got, err := v.LoadSecret("totp", "alice")
	if err != nil {
		t.Fatalf("load secret: %v", err)
	}
	if got != "SECRET1" {
		t.Fatalf("secret mismatch: got %q", got)
	}

	keys, err := v.ListKeys("totp")
	if err != nil {
		t.Fatalf("list keys: %v", err)
	}
	if len(keys) != 1 || keys[0] != "alice" {
		t.Fatalf("unexpected keys: %#v", keys)
	}

	if err := v.DeleteSecret("totp", "alice"); err != nil {
		t.Fatalf("delete secret: %v", err)
	}
	if _, err := v.LoadSecret("totp", "alice"); err == nil {
		t.Fatal("expected missing secret after delete")
	}
}

func TestRuntimeStatusReportsTemporaryEnvMode(t *testing.T) {
	t.Setenv("VAULT_TYPE", "env")

	ActivateTemporarySetup(SetupConfig{
		Mode:           SetupModeTemporary,
		Port:           "8080",
		UserVault:      UserVaultSettings{Type: "env"},
		ServiceSecrets: ServiceSecretConfig{Mode: ServiceSecretMemory},
	})
	activeVault = NewEnvVault()
	store := &UserStore{
		cache: map[string]*User{
			"root": {Username: "root", Role: "admin"},
		},
		filePath: filepath.Join(t.TempDir(), "users.json"),
	}
	globalUserStore = store
	t.Cleanup(func() {
		activeVault = nil
		globalUserStore = nil
	})

	status := GetRuntimeStatus()
	if !status.HasAdmin {
		t.Fatal("expected admin to be present")
	}
	if status.SecretsMode != "temporary" {
		t.Fatalf("expected temporary secrets mode, got %q", status.SecretsMode)
	}
	if status.BootstrapRequired {
		t.Fatal("bootstrap should be disabled when an admin exists")
	}
	if status.SetupRequired {
		t.Fatal("setup should be inactive once temporary setup is activated")
	}
}
