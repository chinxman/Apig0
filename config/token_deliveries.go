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
	"sort"
	"strings"
	"sync"
	"time"
)

type PendingAPITokenDelivery struct {
	ID          string    `json:"id"`
	TokenID     string    `json:"token_id"`
	User        string    `json:"user"`
	TokenPrefix string    `json:"token_prefix"`
	KeyType     string    `json:"key_type,omitempty"`
	Service     string    `json:"service,omitempty"`
	CreatedBy   string    `json:"created_by,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	ExpiresAt   time.Time `json:"expires_at"`
	Nonce       string    `json:"nonce,omitempty"`
	Ciphertext  string    `json:"ciphertext,omitempty"`
}

type pendingTokenDeliveryFile struct {
	Deliveries []PendingAPITokenDelivery `json:"deliveries"`
}

var (
	pendingTokenDeliveryMu   sync.RWMutex
	pendingTokenDeliveryData = map[string]PendingAPITokenDelivery{}
	pendingTokenDeliveryPath = filepath.Join(os.TempDir(), "apig0-token-deliveries-"+randomSuffix()+".json")
	pendingTokenEphemeralKey = mustRandomBytes(32)
)

func pendingTokenDeliveryFilePath() string {
	if path := strings.TrimSpace(os.Getenv("APIG0_TOKEN_DELIVERIES_PATH")); path != "" {
		return path
	}
	if CurrentSetupConfig().Mode == SetupModePersistent {
		return "api-token-deliveries.json"
	}
	return pendingTokenDeliveryPath
}

func LoadPendingAPITokenDeliveries(masterPassword string) error {
	pendingTokenDeliveryMu.Lock()
	defer pendingTokenDeliveryMu.Unlock()

	pendingTokenDeliveryData = map[string]PendingAPITokenDelivery{}
	if !pendingDeliveryPersistenceEnabled(masterPassword) {
		return nil
	}

	raw, err := os.ReadFile(pendingTokenDeliveryFilePath())
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var file pendingTokenDeliveryFile
	if err := json.Unmarshal(raw, &file); err != nil {
		return err
	}

	now := time.Now().UTC()
	for _, delivery := range file.Deliveries {
		delivery = normalizePendingDelivery(delivery)
		if delivery.ID == "" || delivery.ExpiresAt.IsZero() || now.After(delivery.ExpiresAt) {
			continue
		}
		pendingTokenDeliveryData[delivery.ID] = delivery
	}
	return nil
}

func CreatePendingAPITokenDelivery(raw string, token APIToken, createdBy string) (PendingAPITokenDelivery, error) {
	pendingTokenDeliveryMu.Lock()
	defer pendingTokenDeliveryMu.Unlock()

	masterPassword := strings.TrimSpace(os.Getenv("APIG0_SERVICE_MASTER_PASSWORD"))
	delivery, err := buildPendingDelivery(raw, token, createdBy, masterPassword)
	if err != nil {
		return PendingAPITokenDelivery{}, err
	}
	purgeExpiredDeliveriesLocked(time.Now().UTC())
	pendingTokenDeliveryData[delivery.ID] = delivery
	if err := savePendingAPITokenDeliveriesLocked(masterPassword); err != nil {
		delete(pendingTokenDeliveryData, delivery.ID)
		return PendingAPITokenDelivery{}, err
	}
	return delivery, nil
}

func ListPendingAPITokenDeliveriesForUser(user string) []PendingAPITokenDelivery {
	pendingTokenDeliveryMu.Lock()
	defer pendingTokenDeliveryMu.Unlock()

	user = strings.TrimSpace(user)
	if user == "" {
		return nil
	}

	purgeExpiredDeliveriesLocked(time.Now().UTC())
	out := make([]PendingAPITokenDelivery, 0)
	for _, delivery := range pendingTokenDeliveryData {
		if delivery.User != user {
			continue
		}
		out = append(out, pendingDeliveryPublicView(delivery))
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	return out
}

func ClaimPendingAPITokenDelivery(user, id string) (string, PendingAPITokenDelivery, error) {
	pendingTokenDeliveryMu.Lock()
	defer pendingTokenDeliveryMu.Unlock()

	user = strings.TrimSpace(user)
	id = strings.TrimSpace(id)
	if user == "" || id == "" {
		return "", PendingAPITokenDelivery{}, errors.New("pending key not found")
	}

	purgeExpiredDeliveriesLocked(time.Now().UTC())
	delivery, ok := pendingTokenDeliveryData[id]
	if !ok || delivery.User != user {
		return "", PendingAPITokenDelivery{}, errors.New("pending key not found")
	}
	token, ok := GetAPITokenByID(delivery.TokenID)
	if !ok || !token.RevokedAt.IsZero() || (!token.ExpiresAt.IsZero() && time.Now().After(token.ExpiresAt)) {
		delete(pendingTokenDeliveryData, delivery.ID)
		_ = savePendingAPITokenDeliveriesLocked(strings.TrimSpace(os.Getenv("APIG0_SERVICE_MASTER_PASSWORD")))
		return "", PendingAPITokenDelivery{}, errors.New("pending key is no longer active")
	}

	raw, err := decryptPendingDelivery(delivery, strings.TrimSpace(os.Getenv("APIG0_SERVICE_MASTER_PASSWORD")))
	if err != nil {
		return "", PendingAPITokenDelivery{}, err
	}
	delete(pendingTokenDeliveryData, delivery.ID)
	if err := savePendingAPITokenDeliveriesLocked(strings.TrimSpace(os.Getenv("APIG0_SERVICE_MASTER_PASSWORD"))); err != nil {
		return "", PendingAPITokenDelivery{}, err
	}
	return raw, pendingDeliveryPublicView(delivery), nil
}

func DeletePendingAPITokenDeliveriesByTokenID(tokenID string) error {
	pendingTokenDeliveryMu.Lock()
	defer pendingTokenDeliveryMu.Unlock()

	tokenID = strings.TrimSpace(tokenID)
	if tokenID == "" {
		return nil
	}

	changed := false
	for id, delivery := range pendingTokenDeliveryData {
		if delivery.TokenID != tokenID {
			continue
		}
		delete(pendingTokenDeliveryData, id)
		changed = true
	}
	if !changed {
		return nil
	}
	return savePendingAPITokenDeliveriesLocked(strings.TrimSpace(os.Getenv("APIG0_SERVICE_MASTER_PASSWORD")))
}

func savePendingAPITokenDeliveriesLocked(masterPassword string) error {
	if !pendingDeliveryPersistenceEnabled(masterPassword) {
		return nil
	}

	records := make([]PendingAPITokenDelivery, 0, len(pendingTokenDeliveryData))
	for _, delivery := range pendingTokenDeliveryData {
		records = append(records, delivery)
	}
	sort.Slice(records, func(i, j int) bool {
		return records[i].CreatedAt.Before(records[j].CreatedAt)
	})
	raw, err := json.MarshalIndent(pendingTokenDeliveryFile{Deliveries: records}, "", "  ")
	if err != nil {
		return err
	}

	path := pendingTokenDeliveryFilePath()
	if err := ensureParentDir(path); err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0600)
}

func pendingDeliveryPersistenceEnabled(masterPassword string) bool {
	return CurrentSetupConfig().Mode == SetupModePersistent && strings.TrimSpace(masterPassword) != ""
}

func purgeExpiredDeliveriesLocked(now time.Time) {
	changed := false
	for id, delivery := range pendingTokenDeliveryData {
		if delivery.ExpiresAt.IsZero() || now.After(delivery.ExpiresAt) {
			delete(pendingTokenDeliveryData, id)
			changed = true
		}
	}
	if changed {
		_ = savePendingAPITokenDeliveriesLocked(strings.TrimSpace(os.Getenv("APIG0_SERVICE_MASTER_PASSWORD")))
	}
}

func normalizePendingDelivery(delivery PendingAPITokenDelivery) PendingAPITokenDelivery {
	delivery.ID = strings.TrimSpace(delivery.ID)
	delivery.TokenID = strings.TrimSpace(delivery.TokenID)
	delivery.User = strings.TrimSpace(delivery.User)
	delivery.TokenPrefix = strings.TrimSpace(delivery.TokenPrefix)
	delivery.KeyType = normalizeTokenKeyType(delivery.KeyType)
	delivery.Service = NormalizeAllowedServiceName(delivery.Service)
	delivery.CreatedBy = strings.TrimSpace(delivery.CreatedBy)
	delivery.Nonce = strings.TrimSpace(delivery.Nonce)
	delivery.Ciphertext = strings.TrimSpace(delivery.Ciphertext)
	return delivery
}

func pendingDeliveryPublicView(delivery PendingAPITokenDelivery) PendingAPITokenDelivery {
	delivery = normalizePendingDelivery(delivery)
	delivery.Nonce = ""
	delivery.Ciphertext = ""
	return delivery
}

func buildPendingDelivery(raw string, token APIToken, createdBy, masterPassword string) (PendingAPITokenDelivery, error) {
	nonce, ciphertext, err := encryptPendingDelivery(raw, masterPassword)
	if err != nil {
		return PendingAPITokenDelivery{}, err
	}
	now := time.Now().UTC()
	return normalizePendingDelivery(PendingAPITokenDelivery{
		ID:          randomHex(8),
		TokenID:     token.ID,
		User:        token.User,
		TokenPrefix: token.TokenPrefix,
		KeyType:     token.KeyType,
		Service:     token.OpenAIService,
		CreatedBy:   strings.TrimSpace(createdBy),
		CreatedAt:   now,
		ExpiresAt:   now.Add(24 * time.Hour),
		Nonce:       nonce,
		Ciphertext:  ciphertext,
	}), nil
}

func encryptPendingDelivery(raw, masterPassword string) (string, string, error) {
	key := pendingTokenDeliveryKey(masterPassword)
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", "", err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return "", "", err
	}
	nonce := mustRandomBytes(aead.NonceSize())
	ciphertext := aead.Seal(nil, nonce, []byte(raw), nil)
	return base64.StdEncoding.EncodeToString(nonce), base64.StdEncoding.EncodeToString(ciphertext), nil
}

func decryptPendingDelivery(delivery PendingAPITokenDelivery, masterPassword string) (string, error) {
	key := pendingTokenDeliveryKey(masterPassword)
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce, err := base64.StdEncoding.DecodeString(delivery.Nonce)
	if err != nil {
		return "", err
	}
	ciphertext, err := base64.StdEncoding.DecodeString(delivery.Ciphertext)
	if err != nil {
		return "", err
	}
	plain, err := aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

func pendingTokenDeliveryKey(masterPassword string) []byte {
	if strings.TrimSpace(masterPassword) == "" {
		return pendingTokenEphemeralKey
	}
	sum := sha256.Sum256([]byte("apig0-token-delivery:" + masterPassword))
	return sum[:]
}

func mustRandomBytes(length int) []byte {
	out := make([]byte, length)
	if _, err := rand.Read(out); err != nil {
		panic(err)
	}
	return out
}
