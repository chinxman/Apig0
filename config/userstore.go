package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const userVaultPath = "apig0-users"

type User struct {
	Username                string    `json:"username"`
	PassHash                string    `json:"pass_hash"`
	Role                    string    `json:"role"` // "admin" | "user"
	ServiceAccessConfigured bool      `json:"service_access_configured,omitempty"`
	AllowedServices         []string  `json:"allowed_services,omitempty"`
	CreatedAt               time.Time `json:"created_at"`
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
// It tries Vault first for proper namespaced backends (hashicorp, aws, etc.).
// env and file vaults share a flat namespace with TOTP secrets, so user data
// would collide — those backends always fall back to the dedicated file store.
func InitUserStore(filePath string) {
	s := &UserStore{
		cache:    make(map[string]*User),
		filePath: filePath,
	}

	if activeVault != nil {
		vaultName := activeVault.String()
		useForUsers := vaultName != "env" && vaultName != "file"
		if useForUsers {
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

func (s *UserStore) Backend() string {
	if s == nil {
		return ""
	}
	return s.backend()
}

func (s *UserStore) FilePath() string {
	if s == nil {
		return ""
	}
	return s.filePath
}

// ─── Public API ─────────────────────────────────────────────────────────────

func GetUserStore() *UserStore { return globalUserStore }

func (s *UserStore) Create(username, password, role string, allowedServices []string, restrictServices bool) error {
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
		Username:                username,
		PassHash:                string(hash),
		Role:                    role,
		ServiceAccessConfigured: restrictServices,
		AllowedServices:         NormalizeAllowedServices(allowedServices),
		CreatedAt:               time.Now(),
	}
	if err := s.persist(u); err != nil {
		return err
	}
	s.cache[username] = u
	return nil
}

// CreateWithHash adds a user with a pre-hashed password (used during bootstrap
// when passwords are already hashed from env vars).
func (s *UserStore) CreateWithHash(username, passHash, role string, allowedServices []string, restrictServices bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.cache[username]; exists {
		return fmt.Errorf("user %q already exists", username)
	}
	u := &User{
		Username:                username,
		PassHash:                passHash,
		Role:                    role,
		ServiceAccessConfigured: restrictServices,
		AllowedServices:         NormalizeAllowedServices(allowedServices),
		CreatedAt:               time.Now(),
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

	u, exists := s.cache[username]
	if !exists {
		return fmt.Errorf("user %q not found", username)
	}

	if s.useVault {
		if err := s.remove(username); err != nil {
			return err
		}
		delete(s.cache, username)
		return nil
	}

	delete(s.cache, username)
	if err := s.saveToFile(); err != nil {
		s.cache[username] = u
		return err
	}
	return nil
}

func (s *UserStore) List() []User {
	s.mu.RLock()
	defer s.mu.RUnlock()

	users := make([]User, 0, len(s.cache))
	for _, u := range s.cache {
		users = append(users, *u)
	}
	sort.Slice(users, func(i, j int) bool {
		if users[i].CreatedAt.Equal(users[j].CreatedAt) {
			return users[i].Username < users[j].Username
		}
		return users[i].CreatedAt.Before(users[j].CreatedAt)
	})
	return users
}

func (s *UserStore) IsProtectedAdmin(username string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	username = strings.TrimSpace(username)
	if username == "" {
		return false
	}

	var protected *User
	for _, user := range s.cache {
		if user.Role != "admin" {
			continue
		}
		if protected == nil ||
			user.CreatedAt.Before(protected.CreatedAt) ||
			(user.CreatedAt.Equal(protected.CreatedAt) && user.Username < protected.Username) {
			protected = user
		}
	}

	return protected != nil && protected.Username == username
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

func (s *UserStore) GetAllowedServices(username string) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if u, ok := s.cache[username]; ok {
		if !u.ServiceAccessConfigured {
			return nil
		}
		return append([]string(nil), u.AllowedServices...)
	}
	return nil
}

func (s *UserStore) SetAllowedServices(username string, allowedServices []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	u, ok := s.cache[username]
	if !ok {
		return fmt.Errorf("user %q not found", username)
	}

	u.ServiceAccessConfigured = true
	u.AllowedServices = NormalizeAllowedServices(allowedServices)
	return s.persist(u)
}

func (s *UserStore) CanAccessService(username, service string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	u, ok := s.cache[username]
	if !ok {
		return false
	}
	if u.Role == "admin" {
		return true
	}
	if !u.ServiceAccessConfigured {
		return true
	}
	if len(u.AllowedServices) == 0 {
		return false
	}
	for _, allowed := range u.AllowedServices {
		if allowed == service {
			return true
		}
	}
	return false
}

func (s *UserStore) HasConfiguredServiceAccess(username string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if u, ok := s.cache[username]; ok {
		return u.ServiceAccessConfigured
	}
	return false
}

func (s *UserStore) ClearServiceAccessRestrictions(username string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	u, ok := s.cache[username]
	if !ok {
		return fmt.Errorf("user %q not found", username)
	}

	u.ServiceAccessConfigured = false
	u.AllowedServices = nil
	return s.persist(u)
}

func (s *UserStore) RemoveAllowedServiceEverywhere(service string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	service = NormalizeAllowedServiceName(service)
	if service == "" {
		return nil
	}

	changedUsers := make([]*User, 0)
	for _, u := range s.cache {
		if u.Role == "admin" || !u.ServiceAccessConfigured || len(u.AllowedServices) == 0 {
			continue
		}

		next := make([]string, 0, len(u.AllowedServices))
		changed := false
		for _, allowed := range u.AllowedServices {
			if NormalizeAllowedServiceName(allowed) == service {
				changed = true
				continue
			}
			next = append(next, allowed)
		}
		if !changed {
			continue
		}

		u.AllowedServices = next
		changedUsers = append(changedUsers, u)
	}

	if len(changedUsers) == 0 {
		return nil
	}

	if s.useVault {
		for _, u := range changedUsers {
			raw, err := json.Marshal(u)
			if err != nil {
				return err
			}
			if err := activeVault.StoreSecret(userVaultPath, u.Username, string(raw)); err != nil {
				return err
			}
		}
		return nil
	}

	return s.saveToFile()
}

func (s *UserStore) Exists(username string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.cache[username]
	return ok
}

func NormalizeAllowedServiceName(service string) string {
	return strings.TrimSpace(strings.ToLower(service))
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
	delete(s.cache, username)
	return s.saveToFile()
}
