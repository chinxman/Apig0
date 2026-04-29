package config

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

var ErrServiceSecretsLocked = errors.New("service secrets are locked")

type ServiceSecretMode string

const (
	ServiceSecretMemory        ServiceSecretMode = "memory"
	ServiceSecretFile          ServiceSecretMode = "file"
	ServiceSecretEncryptedFile ServiceSecretMode = "encrypted_file"
)

type ServiceSecretConfig struct {
	Mode       ServiceSecretMode `json:"mode"`
	FilePath   string            `json:"file_path,omitempty"`
	Locked     bool              `json:"locked"`
	MasterHint string            `json:"master_hint,omitempty"`
}

type serviceSecretFile struct {
	Secrets map[string]string `json:"secrets"`
}

type encryptedSecretFile struct {
	Salt       string `json:"salt"`
	Nonce      string `json:"nonce"`
	Ciphertext string `json:"ciphertext"`
}

var (
	serviceSecretMu     sync.RWMutex
	serviceSecretConfig = ServiceSecretConfig{Mode: ServiceSecretMemory}
	serviceSecrets      = map[string]string{}
)

func NormalizeTemporaryServiceSecretConfig(cfg ServiceSecretConfig) ServiceSecretConfig {
	cfg.Mode = ServiceSecretMemory
	cfg.FilePath = ""
	cfg.Locked = false
	cfg.MasterHint = ""
	return cfg
}

func NormalizePersistentServiceSecretConfig(cfg ServiceSecretConfig) ServiceSecretConfig {
	cfg.Locked = false
	cfg.MasterHint = ""

	switch cfg.Mode {
	case ServiceSecretEncryptedFile:
		if strings.TrimSpace(cfg.FilePath) == "" {
			cfg.FilePath = "service-secrets.enc.json"
		}
	default:
		cfg.Mode = ServiceSecretFile
		if strings.TrimSpace(cfg.FilePath) == "" || cfg.FilePath == "service-secrets.enc.json" {
			cfg.FilePath = "service-secrets.json"
		}
	}

	return cfg
}

func ConfigureServiceSecrets(cfg ServiceSecretConfig, initial map[string]string, masterPassword string) error {
	serviceSecretMu.Lock()
	defer serviceSecretMu.Unlock()

	if cfg.Mode == "" {
		cfg.Mode = ServiceSecretMemory
	}
	if cfg.Mode == ServiceSecretEncryptedFile && cfg.FilePath != "" {
		existing, locked, err := loadEncryptedServiceSecrets(cfg.FilePath, masterPassword)
		if err != nil {
			return err
		}
		cfg.Locked = locked
		serviceSecrets = existing
	} else if cfg.Mode == ServiceSecretFile && cfg.FilePath != "" {
		existing, err := loadPlainServiceSecrets(cfg.FilePath)
		if err != nil {
			return err
		}
		serviceSecrets = existing
	} else {
		serviceSecrets = map[string]string{}
	}

	if initial != nil {
		for k, v := range initial {
			serviceSecrets[k] = v
		}
	}
	serviceSecretConfig = cfg
	return persistServiceSecretsLocked(masterPassword)
}

func ServiceSecretStatus() ServiceSecretConfig {
	serviceSecretMu.RLock()
	defer serviceSecretMu.RUnlock()
	return serviceSecretConfig
}

func SetServiceSecret(name, secret string) error {
	serviceSecretMu.Lock()
	defer serviceSecretMu.Unlock()

	if serviceSecretConfig.Locked {
		return ErrServiceSecretsLocked
	}
	serviceSecrets[name] = secret
	return persistServiceSecretsLocked(os.Getenv("APIG0_SERVICE_MASTER_PASSWORD"))
}

func DeleteServiceSecret(name string) error {
	serviceSecretMu.Lock()
	defer serviceSecretMu.Unlock()

	if serviceSecretConfig.Locked {
		return ErrServiceSecretsLocked
	}
	delete(serviceSecrets, name)
	return persistServiceSecretsLocked(os.Getenv("APIG0_SERVICE_MASTER_PASSWORD"))
}

func GetServiceSecret(name string) (string, bool, error) {
	serviceSecretMu.RLock()
	defer serviceSecretMu.RUnlock()

	if serviceSecretConfig.Locked {
		return "", false, ErrServiceSecretsLocked
	}
	val, ok := serviceSecrets[name]
	return val, ok, nil
}

func persistServiceSecretsLocked(masterPassword string) error {
	switch serviceSecretConfig.Mode {
	case ServiceSecretMemory:
		return nil
	case ServiceSecretFile:
		return savePlainServiceSecrets(serviceSecretConfig.FilePath, serviceSecrets)
	case ServiceSecretEncryptedFile:
		if masterPassword == "" {
			return nil
		}
		serviceSecretConfig.Locked = false
		return saveEncryptedServiceSecrets(serviceSecretConfig.FilePath, serviceSecrets, masterPassword)
	default:
		return nil
	}
}

func loadPlainServiceSecrets(path string) (map[string]string, error) {
	out := map[string]string{}
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return out, nil
		}
		return nil, err
	}
	var file serviceSecretFile
	if err := json.Unmarshal(raw, &file); err != nil {
		return nil, err
	}
	if file.Secrets != nil {
		out = file.Secrets
	}
	return out, nil
}

func savePlainServiceSecrets(path string, secrets map[string]string) error {
	if path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil && filepath.Dir(path) != "." {
		return err
	}
	raw, err := json.MarshalIndent(serviceSecretFile{Secrets: secrets}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0600)
}

func loadEncryptedServiceSecrets(path, masterPassword string) (map[string]string, bool, error) {
	out := map[string]string{}
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return out, false, nil
		}
		return nil, false, err
	}
	if masterPassword == "" {
		return out, true, nil
	}

	var file encryptedSecretFile
	if err := json.Unmarshal(raw, &file); err != nil {
		return nil, false, err
	}
	salt, err := base64.StdEncoding.DecodeString(file.Salt)
	if err != nil {
		return nil, false, err
	}
	nonce, err := base64.StdEncoding.DecodeString(file.Nonce)
	if err != nil {
		return nil, false, err
	}
	ciphertext, err := base64.StdEncoding.DecodeString(file.Ciphertext)
	if err != nil {
		return nil, false, err
	}

	key := deriveServiceSecretKey(masterPassword, salt)
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, false, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, false, err
	}
	plain, err := aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, false, err
	}
	var fileData serviceSecretFile
	if err := json.Unmarshal(plain, &fileData); err != nil {
		return nil, false, err
	}
	if fileData.Secrets != nil {
		out = fileData.Secrets
	}
	return out, false, nil
}

func saveEncryptedServiceSecrets(path string, secrets map[string]string, masterPassword string) error {
	if path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil && filepath.Dir(path) != "." {
		return err
	}

	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return err
	}
	nonce := make([]byte, 12)
	if _, err := rand.Read(nonce); err != nil {
		return err
	}

	key := deriveServiceSecretKey(masterPassword, salt)
	block, err := aes.NewCipher(key)
	if err != nil {
		return err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}
	plain, err := json.Marshal(serviceSecretFile{Secrets: secrets})
	if err != nil {
		return err
	}
	file := encryptedSecretFile{
		Salt:       base64.StdEncoding.EncodeToString(salt),
		Nonce:      base64.StdEncoding.EncodeToString(nonce),
		Ciphertext: base64.StdEncoding.EncodeToString(aead.Seal(nil, nonce, plain, nil)),
	}
	raw, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0600)
}

func deriveServiceSecretKey(masterPassword string, salt []byte) []byte {
	sum := sha256.Sum256(append([]byte(masterPassword), salt...))
	return sum[:]
}
