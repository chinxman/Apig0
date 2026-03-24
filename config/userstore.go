package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const userVaultPath = "apig0-users"

type User struct {
	Username  string    `json:"username"`
	PassHash  string    `json:"pass_hash"`
	Role      string    `json:"role"` // "admin" | "user"
	CreatedAt time.Time `json:"created_at"`
}

// UserStore holds a fast in-memory cache of users and writes through to
// either HashiCorp Vault (when available) or a local JSON file as fallback.
type UserStore struct {
	mu       sync.RWMutex
	cache    map[string]*User
	useVault bool   // true = Vault is the source of truth
	filePath string // used only in file-fallback mode
}

var globalUserStore *UserStore

// InitUserStore is called after InitSecrets so activeVault is ready.
// It tries Vault first; if the vault is read-only it falls back to a local file.
func InitUserStore(filePath string) {
	s := &UserStore{
		cache:    make(map[string]*User),
		filePath: filePath,
	}

	if activeVault != nil {
		if err := s.loadFromVault(); err != nil {
			if !errors.Is(err, ErrReadOnly) {
				log.Printf("[userstore] vault unavailable (%v), falling back to file", err)
			} else {
				log.Printf("[userstore] vault is read-only, using file store")
			}
		} else {
			s.useVault = true
		}
	}

	if !s.useVault {
		s.loadFromFile()
	}

	globalUserStore = s
	log.Printf("[userstore] ready — %d users (backend: %s)", len(s.cache), s.backend())
}

func (s *UserStore) backend() string {
	if s.useVault {
		return activeVault.String()
	}
	return "file"
}

// ─── Public API ─────────────────────────────────────────────────────────────

func GetUserStore() *UserStore { return globalUserStore }

func (s *UserStore) Create(username, password, role string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.cache[username]; exists {
		return fmt.Errorf("user %q already exists", username)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	u := &User{
		Username:  username,
		PassHash:  string(hash),
		Role:      role,
		CreatedAt: time.Now(),
	}
	if err := s.persist(u); err != nil {
		return err
	}
	s.cache[username] = u
	return nil
}

func (s *UserStore) Delete(username string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.cache[username]; !exists {
		return fmt.Errorf("user %q not found", username)
	}
	if err := s.remove(username); err != nil {
		return err
	}
	delete(s.cache, username)
	return nil
}

func (s *UserStore) List() []User {
	s.mu.RLock()
	defer s.mu.RUnlock()

	users := make([]User, 0, len(s.cache))
	for _, u := range s.cache {
		users = append(users, *u)
	}
	return users
}

func (s *UserStore) ValidatePassword(username, password string) bool {
	s.mu.RLock()
	u, ok := s.cache[username]
	s.mu.RUnlock()
	if !ok {
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(u.PassHash), []byte(password)) == nil
}

func (s *UserStore) GetRole(username string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if u, ok := s.cache[username]; ok {
		return u.Role
	}
	return ""
}

func (s *UserStore) Exists(username string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.cache[username]
	return ok
}

// ─── Vault backend ──────────────────────────────────────────────────────────

func (s *UserStore) loadFromVault() error {
	keys, err := activeVault.ListKeys(userVaultPath)
	if err != nil {
		return err
	}
	for _, key := range keys {
		raw, err := activeVault.LoadSecret(userVaultPath, key)
		if err != nil {
			log.Printf("[userstore] could not load user %q from vault: %v", key, err)
			continue
		}
		var u User
		if err := json.Unmarshal([]byte(raw), &u); err != nil {
			log.Printf("[userstore] could not decode user %q: %v", key, err)
			continue
		}
		s.cache[key] = &u
	}
	return nil
}

// ─── File backend ───────────────────────────────────────────────────────────

type fileStoreData struct {
	Users map[string]*User `json:"users"`
}

func (s *UserStore) loadFromFile() {
	raw, err := os.ReadFile(s.filePath)
	if err != nil {
		return
	}
	var d fileStoreData
	if err := json.Unmarshal(raw, &d); err != nil {
		log.Printf("[userstore] failed to parse %s: %v", s.filePath, err)
		return
	}
	if d.Users != nil {
		s.cache = d.Users
	}
}

func (s *UserStore) saveToFile() error {
	d := fileStoreData{Users: s.cache}
	raw, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.filePath, raw, 0600)
}

// ─── Write-through helpers ───────────────────────────────────────────────────

func (s *UserStore) persist(u *User) error {
	if s.useVault {
		raw, err := json.Marshal(u)
		if err != nil {
			return err
		}
		return activeVault.StoreSecret(userVaultPath, u.Username, string(raw))
	}
	// File mode: update cache first, then flush
	s.cache[u.Username] = u
	return s.saveToFile()
}

func (s *UserStore) remove(username string) error {
	if s.useVault {
		return activeVault.DeleteSecret(userVaultPath, username)
	}
	return s.saveToFile()
}
