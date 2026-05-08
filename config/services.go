package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"sort"
	"strings"
	"sync"
)

type ServiceAuthType string

const (
	ServiceAuthNone         ServiceAuthType = "none"
	ServiceAuthBearer       ServiceAuthType = "bearer"
	ServiceAuthXAPIKey      ServiceAuthType = "x-api-key"
	ServiceAuthCustomHeader ServiceAuthType = "custom-header"
	ServiceAuthBasic        ServiceAuthType = "basic"
)

type ServiceConfig struct {
	Name          string          `json:"name"`
	BaseURL       string          `json:"base_url"`
	AuthType      ServiceAuthType `json:"auth_type"`
	HeaderName    string          `json:"header_name,omitempty"`
	BasicUsername string          `json:"basic_username,omitempty"`
	Provider      string          `json:"provider,omitempty"`
	OpenAICompat  bool            `json:"openai_compatible,omitempty"`
	TimeoutMS     int             `json:"timeout_ms,omitempty"`
	RetryCount    int             `json:"retry_count,omitempty"`
	Enabled       bool            `json:"enabled"`
	HasSecret     bool            `json:"has_secret"`
}

type serviceFileData struct {
	Services []ServiceConfig `json:"services"`
}

var (
	serviceMu   sync.RWMutex
	serviceData = map[string]ServiceConfig{}
	servicePath = "services.json"
)

func DefaultDemoServices() []ServiceConfig {
	return []ServiceConfig{
		{Name: "demo-users", BaseURL: "https://httpbin.org/anything/demo-users", AuthType: ServiceAuthNone, Enabled: true},
		{Name: "demo-orders", BaseURL: "https://httpbin.org/anything/demo-orders", AuthType: ServiceAuthNone, Enabled: true},
	}
}

func LoadServices() map[string]ServiceConfig {
	path := os.Getenv("APIG0_SERVICES_PATH")
	if path != "" {
		servicePath = path
	}

	serviceMu.Lock()
	defer serviceMu.Unlock()

	serviceData = map[string]ServiceConfig{}
	raw, err := os.ReadFile(servicePath)
	if err != nil {
		return cloneServiceMap(serviceData)
	}

	var fileData serviceFileData
	if err := json.Unmarshal(raw, &fileData); err != nil {
		return cloneServiceMap(serviceData)
	}

	for _, svc := range fileData.Services {
		normalized, ok := normalizeServiceConfig(svc)
		if !ok {
			continue
		}
		serviceData[normalized.Name] = normalized
	}
	return cloneServiceMap(serviceData)
}

func SaveServices(services []ServiceConfig) error {
	normalized := make([]ServiceConfig, 0, len(services))
	for _, svc := range services {
		clean, ok := normalizeServiceConfig(svc)
		if !ok {
			continue
		}
		normalized = append(normalized, clean)
	}
	sort.Slice(normalized, func(i, j int) bool { return normalized[i].Name < normalized[j].Name })

	serviceMu.Lock()
	defer serviceMu.Unlock()

	serviceData = map[string]ServiceConfig{}
	for _, svc := range normalized {
		serviceData[svc.Name] = svc
	}

	raw, err := json.MarshalIndent(serviceFileData{Services: normalized}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(servicePath, raw, 0600)
}

func SetServicesInMemory(services []ServiceConfig) {
	serviceMu.Lock()
	defer serviceMu.Unlock()

	serviceData = map[string]ServiceConfig{}
	for _, svc := range services {
		normalized, ok := normalizeServiceConfig(svc)
		if !ok {
			continue
		}
		serviceData[normalized.Name] = normalized
	}
}

func GetServices() map[string]string {
	serviceMu.RLock()
	defer serviceMu.RUnlock()

	out := make(map[string]string, len(serviceData))
	for name, svc := range serviceData {
		if !svc.Enabled {
			continue
		}
		out[name] = svc.BaseURL
	}
	return out
}

func GetServiceCatalog() []ServiceConfig {
	serviceMu.RLock()
	defer serviceMu.RUnlock()

	out := make([]ServiceConfig, 0, len(serviceData))
	for _, svc := range serviceData {
		out = append(out, svc)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func GetServiceConfig(name string) (ServiceConfig, bool) {
	name = strings.TrimSpace(strings.ToLower(name))

	serviceMu.RLock()
	defer serviceMu.RUnlock()

	svc, ok := serviceData[name]
	return svc, ok
}

func LookupService(name string) (ServiceConfig, bool) {
	serviceMu.RLock()
	defer serviceMu.RUnlock()

	svc, ok := serviceData[name]
	if !ok || !svc.Enabled {
		return ServiceConfig{}, false
	}
	return svc, true
}

func ListServiceNames() []string {
	serviceMu.RLock()
	defer serviceMu.RUnlock()

	names := make([]string, 0, len(serviceData))
	for name, svc := range serviceData {
		if svc.Enabled {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

func ListOpenAICompatibleServiceNames() []string {
	serviceMu.RLock()
	defer serviceMu.RUnlock()

	names := make([]string, 0, len(serviceData))
	for name, svc := range serviceData {
		if svc.Enabled && svc.OpenAICompat {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

func IsKnownService(name string) bool {
	serviceMu.RLock()
	defer serviceMu.RUnlock()
	svc, ok := serviceData[name]
	return ok && svc.Enabled
}

func NormalizeAllowedServices(services []string) []string {
	if len(services) == 0 {
		return []string{}
	}

	seen := make(map[string]struct{}, len(services))
	out := make([]string, 0, len(services))
	for _, service := range services {
		service = strings.TrimSpace(service)
		if !IsKnownService(service) {
			continue
		}
		if _, ok := seen[service]; ok {
			continue
		}
		seen[service] = struct{}{}
		out = append(out, service)
	}
	sort.Strings(out)
	return out
}

func cloneServiceMap(src map[string]ServiceConfig) map[string]ServiceConfig {
	out := make(map[string]ServiceConfig, len(src))
	for k, v := range src {
		out[k] = v
	}
	return out
}

func UpsertService(svc ServiceConfig) error {
	clean, ok := normalizeServiceConfig(svc)
	if !ok {
		return os.ErrInvalid
	}

	services := GetServiceCatalog()
	replaced := false
	for i := range services {
		if services[i].Name == clean.Name {
			services[i] = clean
			replaced = true
			break
		}
	}
	if !replaced {
		services = append(services, clean)
	}
	return SaveServices(services)
}

func DeleteService(name string) error {
	name = strings.TrimSpace(strings.ToLower(name))
	if name == "" {
		return os.ErrInvalid
	}

	services := GetServiceCatalog()
	filtered := make([]ServiceConfig, 0, len(services))
	removed := false
	for _, svc := range services {
		if svc.Name == name {
			removed = true
			continue
		}
		filtered = append(filtered, svc)
	}
	if !removed {
		return os.ErrNotExist
	}
	return SaveServices(filtered)
}

func normalizeServiceConfig(svc ServiceConfig) (ServiceConfig, bool) {
	svc.Name = strings.TrimSpace(strings.ToLower(svc.Name))
	svc.BaseURL = strings.TrimSpace(svc.BaseURL)
	svc.HeaderName = strings.TrimSpace(svc.HeaderName)
	svc.BasicUsername = strings.TrimSpace(svc.BasicUsername)
	svc.Provider = NormalizeProviderName(svc.Provider)
	if svc.Name == "" || svc.BaseURL == "" {
		return ServiceConfig{}, false
	}
	if ValidateServiceBaseURL(svc.BaseURL) != nil {
		return ServiceConfig{}, false
	}
	switch svc.AuthType {
	case ServiceAuthNone, ServiceAuthBearer, ServiceAuthXAPIKey, ServiceAuthCustomHeader, ServiceAuthBasic:
	default:
		svc.AuthType = ServiceAuthNone
	}
	if svc.AuthType == ServiceAuthXAPIKey && svc.HeaderName == "" {
		svc.HeaderName = "X-API-Key"
	}
	if svc.AuthType == ServiceAuthCustomHeader && svc.HeaderName == "" {
		svc.HeaderName = "Authorization"
	}
	if svc.TimeoutMS <= 0 {
		svc.TimeoutMS = 10000
	}
	if svc.TimeoutMS > 120000 {
		svc.TimeoutMS = 120000
	}
	if svc.RetryCount < 0 {
		svc.RetryCount = 0
	}
	if svc.RetryCount > 3 {
		svc.RetryCount = 3
	}
	return svc, true
}

func ValidateServiceBaseURL(raw string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return errors.New("base URL is required")
	}

	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("base URL is invalid: %w", err)
	}
	if u.Scheme == "" || u.Host == "" {
		return errors.New("base URL must be an absolute http or https URL")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return errors.New("base URL scheme must be http or https")
	}
	if u.User != nil {
		return errors.New("base URL must not include credentials")
	}
	if strings.TrimSpace(u.Fragment) != "" {
		return errors.New("base URL must not include a fragment")
	}
	return nil
}

func NormalizeProviderName(raw string) string {
	return strings.TrimSpace(strings.ToLower(raw))
}
