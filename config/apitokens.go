package config

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type APIToken struct {
	ID               string    `json:"id"`
	Name             string    `json:"name"`
	User             string    `json:"user"`
	KeyType          string    `json:"key_type,omitempty"`
	TokenPrefix      string    `json:"token_prefix"`
	AllowedServices  []string  `json:"allowed_services,omitempty"`
	OpenAIService    string    `json:"openai_service,omitempty"`
	AllowedModels    []string  `json:"allowed_models,omitempty"`
	AllowedProviders []string  `json:"allowed_providers,omitempty"`
	RateLimitRPM     int       `json:"rate_limit_rpm,omitempty"`
	RateLimitBurst   int       `json:"rate_limit_burst,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
	ExpiresAt        time.Time `json:"expires_at,omitempty"`
	LastUsedAt       time.Time `json:"last_used_at,omitempty"`
	RevokedAt        time.Time `json:"revoked_at,omitempty"`
}

type APITokenCreateParams struct {
	Name             string
	User             string
	KeyType          string
	AllowedServices  []string
	OpenAIService    string
	AllowedModels    []string
	AllowedProviders []string
	RateLimitRPM     int
	RateLimitBurst   int
	ExpiresAt        time.Time
}

type apiTokenRecord struct {
	APIToken
	TokenHash string `json:"token_hash"`
}

type apiTokenFile struct {
	Tokens []apiTokenRecord `json:"tokens"`
}

var (
	apiTokenMu   sync.RWMutex
	apiTokenData = map[string]apiTokenRecord{}
	apiTokenPath = filepath.Join(os.TempDir(), "apig0-api-tokens-"+randomSuffix()+".json")
)

func apiTokensFilePath() string {
	if path := strings.TrimSpace(os.Getenv("APIG0_API_TOKENS_PATH")); path != "" {
		return path
	}
	if CurrentSetupConfig().Mode == SetupModePersistent {
		return "api-tokens.json"
	}
	return apiTokenPath
}

func LoadAPITokens() error {
	apiTokenMu.Lock()
	defer apiTokenMu.Unlock()

	apiTokenData = map[string]apiTokenRecord{}
	raw, err := os.ReadFile(apiTokensFilePath())
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var file apiTokenFile
	if err := json.Unmarshal(raw, &file); err != nil {
		return err
	}
	for _, token := range file.Tokens {
		apiTokenData[token.ID] = normalizeAPITokenRecord(token)
	}
	return nil
}

func ListAPITokens() []APIToken {
	apiTokenMu.RLock()
	defer apiTokenMu.RUnlock()

	out := make([]APIToken, 0, len(apiTokenData))
	for _, token := range apiTokenData {
		out = append(out, token.APIToken)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	return out
}

func APITokenCount() int {
	apiTokenMu.RLock()
	defer apiTokenMu.RUnlock()
	return len(apiTokenData)
}

func CreateAPIToken(params APITokenCreateParams) (string, APIToken, error) {
	apiTokenMu.Lock()
	defer apiTokenMu.Unlock()

	user := strings.TrimSpace(params.User)
	if user == "" {
		return "", APIToken{}, fmt.Errorf("token user is required")
	}

	openAIService := NormalizeAllowedServiceName(params.OpenAIService)
	keyType := normalizeTokenKeyType(params.KeyType)
	if keyType == "ai" && openAIService == "" {
		return "", APIToken{}, fmt.Errorf("AI keys require an AI gateway service")
	}
	if keyType != "ai" {
		openAIService = ""
	}
	if openAIService != "" {
		service, ok := GetServiceConfig(openAIService)
		if !ok || !service.Enabled || !service.OpenAICompat {
			return "", APIToken{}, fmt.Errorf("selected AI gateway service is invalid")
		}
	}
	allowedServices := NormalizeAllowedServices(params.AllowedServices)
	if openAIService != "" && len(allowedServices) > 0 {
		found := false
		for _, service := range allowedServices {
			if service == openAIService {
				found = true
				break
			}
		}
		if !found {
			allowedServices = append(allowedServices, openAIService)
			sort.Strings(allowedServices)
		}
	}

	raw := randomHex(32)
	hash := hashToken(raw)
	now := time.Now().UTC()
	record := normalizeAPITokenRecord(apiTokenRecord{
		APIToken: APIToken{
			ID:               randomHex(8),
			Name:             strings.TrimSpace(params.Name),
			User:             user,
			KeyType:          keyType,
			TokenPrefix:      raw[:12],
			AllowedServices:  allowedServices,
			OpenAIService:    openAIService,
			AllowedModels:    normalizeAllowedModelsForKeyType(params.AllowedModels, keyType),
			AllowedProviders: normalizeAllowedProvidersForKeyType(params.AllowedProviders, keyType),
			RateLimitRPM:     params.RateLimitRPM,
			RateLimitBurst:   params.RateLimitBurst,
			CreatedAt:        now,
			ExpiresAt:        params.ExpiresAt.UTC(),
		},
		TokenHash: hash,
	})
	if record.Name == "" {
		record.Name = "token-" + record.ID[:8]
	}
	apiTokenData[record.ID] = record
	if err := saveAPITokensLocked(); err != nil {
		delete(apiTokenData, record.ID)
		return "", APIToken{}, err
	}
	return raw, record.APIToken, nil
}

func RevokeAPIToken(id string) error {
	apiTokenMu.Lock()
	defer apiTokenMu.Unlock()

	record, ok := apiTokenData[strings.TrimSpace(id)]
	if !ok {
		return os.ErrNotExist
	}
	if record.RevokedAt.IsZero() {
		record.RevokedAt = time.Now().UTC()
		apiTokenData[record.ID] = record
	}
	if err := saveAPITokensLocked(); err != nil {
		return err
	}
	return DeletePendingAPITokenDeliveriesByTokenID(record.ID)
}

func ValidateAPIToken(raw string) (APIToken, bool) {
	hash := hashToken(strings.TrimSpace(raw))

	apiTokenMu.Lock()
	defer apiTokenMu.Unlock()
	for id, token := range apiTokenData {
		if token.TokenHash != hash {
			continue
		}
		if !token.RevokedAt.IsZero() {
			return APIToken{}, false
		}
		if !token.ExpiresAt.IsZero() && time.Now().After(token.ExpiresAt) {
			return APIToken{}, false
		}
		token.LastUsedAt = time.Now().UTC()
		apiTokenData[id] = token
		_ = saveAPITokensLocked()
		return token.APIToken, true
	}
	return APIToken{}, false
}

func GetLatestActiveTokenForUser(user string) (APIToken, bool) {
	apiTokenMu.RLock()
	defer apiTokenMu.RUnlock()

	user = strings.TrimSpace(user)
	if user == "" {
		return APIToken{}, false
	}

	var best apiTokenRecord
	found := false
	now := time.Now()
	for _, token := range apiTokenData {
		if token.User != user {
			continue
		}
		if !token.RevokedAt.IsZero() {
			continue
		}
		if !token.ExpiresAt.IsZero() && now.After(token.ExpiresAt) {
			continue
		}
		if !found || token.CreatedAt.After(best.CreatedAt) {
			best = token
			found = true
		}
	}
	if !found {
		return APIToken{}, false
	}
	return best.APIToken, true
}

func GetAPITokenByID(id string) (APIToken, bool) {
	apiTokenMu.RLock()
	defer apiTokenMu.RUnlock()

	token, ok := apiTokenData[strings.TrimSpace(id)]
	if !ok {
		return APIToken{}, false
	}
	return token.APIToken, true
}

func saveAPITokensLocked() error {
	records := make([]apiTokenRecord, 0, len(apiTokenData))
	for _, token := range apiTokenData {
		records = append(records, token)
	}
	sort.Slice(records, func(i, j int) bool {
		return records[i].CreatedAt.Before(records[j].CreatedAt)
	})
	raw, err := json.MarshalIndent(apiTokenFile{Tokens: records}, "", "  ")
	if err != nil {
		return err
	}
	path := apiTokensFilePath()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil && filepath.Dir(path) != "." {
		return err
	}
	return os.WriteFile(path, raw, 0600)
}

func normalizeAPITokenRecord(record apiTokenRecord) apiTokenRecord {
	record.ID = strings.TrimSpace(record.ID)
	record.Name = strings.TrimSpace(record.Name)
	record.User = strings.TrimSpace(record.User)
	record.KeyType = normalizeTokenKeyType(record.KeyType)
	record.TokenPrefix = strings.TrimSpace(record.TokenPrefix)
	record.TokenHash = strings.TrimSpace(record.TokenHash)
	record.AllowedServices = NormalizeAllowedServices(record.AllowedServices)
	record.OpenAIService = NormalizeAllowedServiceName(record.OpenAIService)
	record.AllowedModels = normalizeAllowedModelsForKeyType(record.AllowedModels, record.KeyType)
	record.AllowedProviders = normalizeAllowedProvidersForKeyType(record.AllowedProviders, record.KeyType)
	override := NormalizeRateLimitRule(RateLimitRule{
		RequestsPerMinute: record.RateLimitRPM,
		Burst:             record.RateLimitBurst,
	})
	record.RateLimitRPM = override.RequestsPerMinute
	record.RateLimitBurst = override.Burst
	return record
}

func hashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func normalizeTokenStringList(values []string, lower bool) []string {
	if len(values) == 0 {
		return []string{}
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if lower {
			value = strings.ToLower(value)
		}
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func normalizeTokenKeyType(raw string) string {
	switch strings.TrimSpace(strings.ToLower(raw)) {
	case "ai":
		return "ai"
	default:
		return "standard"
	}
}

func normalizeAllowedModelsForKeyType(values []string, keyType string) []string {
	if normalizeTokenKeyType(keyType) != "ai" {
		return []string{}
	}
	return normalizeTokenStringList(values, false)
}

func normalizeAllowedProvidersForKeyType(values []string, keyType string) []string {
	if normalizeTokenKeyType(keyType) != "ai" {
		return []string{}
	}
	return normalizeTokenStringList(values, true)
}
