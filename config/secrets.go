package config

import (
	"log"
	"os"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

// UserSecrets contains TOTP secrets for users.
// Secrets can be loaded from vault or environment.
var UserSecrets = map[string]string{}

// UserPasswords contains bcrypt-hashed passwords keyed by username.
// Loaded at startup from APIG0_PASSWORD_<USER> env vars.
var UserPasswords = map[string]string{}

// InitSecrets loads secrets from vault or environment.
// Call this early in main() before using UserSecrets.
// Secrets are NEVER hardcoded — they must come from vault or env vars.
func InitSecrets() {
	LoadVaultSecrets()
	LoadUserPasswords()
	userPath := os.Getenv("APIG0_USERS_PATH")
	if userPath == "" {
		userPath = "users.json"
	}
	InitUserStore(userPath)
	bootstrapUsers()

	if len(UserSecrets) == 0 {
		log.Println("[config] WARNING: no TOTP secrets loaded")
		log.Println("[config] Set APIG0_TOTP_SECRET_<USER> env vars, or configure a vault backend")
	}
}

func configuredUsers() []string {
	raw := strings.TrimSpace(os.Getenv("APIG0_USERS"))
	if raw == "" {
		return nil
	}

	users := make([]string, 0)
	for _, user := range strings.Split(raw, ",") {
		user = strings.TrimSpace(user)
		if user == "" {
			continue
		}
		users = append(users, user)
	}
	return users
}

// bootstrapUsers ensures that every user with a loaded password also exists
// in the UserStore. The first user listed in APIG0_USERS gets the "admin" role;
// all others default to "user". This solves the chicken-and-egg problem where
// env-var passwords are hashed but never registered in the store.
func bootstrapUsers() {
	setup := CurrentSetupConfig()
	if !SetupConfigured() && setup.RequiresSetup {
		return
	}

	store := GetUserStore()
	if store == nil {
		return
	}

	userList := configuredUsers()
	if len(userList) == 0 {
		return
	}

	for i, user := range userList {
		if store.Exists(user) {
			continue
		}
		hash, ok := UserPasswords[user]
		if !ok {
			continue
		}
		role := "user"
		if i == 0 {
			role = "admin"
		}
		store.CreateWithHash(user, hash, role, nil, false)
		log.Printf("[config] bootstrapped user %q (role: %s)", user, role)
	}
}

// LoadUserPasswords reads APIG0_PASSWORD_<USER> for each configured user,
// bcrypt-hashes the plaintext at startup, and stores the hash in UserPasswords.
func LoadUserPasswords() {
	UserPasswords = map[string]string{}
	for _, user := range configuredUsers() {
		envName := "APIG0_PASSWORD_" + strings.ToUpper(strings.ReplaceAll(user, "-", "_"))
		plain := os.Getenv(envName)
		if plain == "" {
			continue
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
		if err != nil {
			log.Printf("[config] failed to hash password for %q: %v", user, err)
			continue
		}
		UserPasswords[user] = string(hash)
		log.Printf("[config] loaded password for %q", user)
	}
}

// ValidatePassword checks a plaintext password. Checks the userstore first
// (vault-backed or file), then falls back to env-var loaded passwords.
func ValidatePassword(user, password string) bool {
	if store := GetUserStore(); store != nil {
		if store.ValidatePassword(user, password) {
			return true
		}
	}
	hash, ok := UserPasswords[user]
	if !ok {
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}
