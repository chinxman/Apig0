package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
)

type AccessPolicyRule struct {
	ID                string   `json:"id"`
	Service           string   `json:"service"`
	PathPrefix        string   `json:"path_prefix"`
	Methods           []string `json:"methods,omitempty"`
	RequireSessionMFA bool     `json:"require_session_mfa,omitempty"`
	Description       string   `json:"description,omitempty"`
}

type AccessDecision struct {
	Allowed  bool   `json:"allowed"`
	Reason   string `json:"reason"`
	PolicyID string `json:"policy_id,omitempty"`
}

type accessPolicyFile struct {
	Users map[string][]AccessPolicyRule `json:"users"`
}

var (
	accessPolicyMu   sync.RWMutex
	accessPolicyData = map[string][]AccessPolicyRule{}
	accessPolicyPath = filepath.Join(os.TempDir(), "apig0-access-policies-"+randomSuffix()+".json")
)

func accessPoliciesFilePath() string {
	if path := strings.TrimSpace(os.Getenv("APIG0_ACCESS_POLICIES_PATH")); path != "" {
		return path
	}
	if CurrentSetupConfig().Mode == SetupModePersistent {
		return "access-policies.json"
	}
	return accessPolicyPath
}

func LoadAccessPolicies() error {
	accessPolicyMu.Lock()
	defer accessPolicyMu.Unlock()

	accessPolicyData = map[string][]AccessPolicyRule{}
	raw, err := os.ReadFile(accessPoliciesFilePath())
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var file accessPolicyFile
	if err := json.Unmarshal(raw, &file); err != nil {
		return err
	}
	for user, rules := range file.Users {
		accessPolicyData[strings.TrimSpace(user)] = normalizeAccessPolicyRules(rules)
	}
	return nil
}

func SetUserAccessPolicies(user string, rules []AccessPolicyRule) error {
	accessPolicyMu.Lock()
	defer accessPolicyMu.Unlock()

	user = strings.TrimSpace(user)
	if user == "" {
		return os.ErrInvalid
	}
	if len(rules) == 0 {
		delete(accessPolicyData, user)
		return saveAccessPoliciesLocked()
	}
	accessPolicyData[user] = normalizeAccessPolicyRules(rules)
	return saveAccessPoliciesLocked()
}

func GetUserAccessPolicies(user string) []AccessPolicyRule {
	accessPolicyMu.RLock()
	defer accessPolicyMu.RUnlock()

	rules := accessPolicyData[strings.TrimSpace(user)]
	out := make([]AccessPolicyRule, len(rules))
	copy(out, rules)
	return out
}

func AccessPolicyUserCount() int {
	accessPolicyMu.RLock()
	defer accessPolicyMu.RUnlock()
	return len(accessPolicyData)
}

func EvaluateRouteAccess(user, service, method, path, authSource string) AccessDecision {
	user = strings.TrimSpace(user)
	service = NormalizeAllowedServiceName(service)
	method = strings.ToUpper(strings.TrimSpace(method))
	path = normalizePolicyPath(path)

	if store := GetUserStore(); store != nil {
		if store.GetRole(user) == "admin" {
			return AccessDecision{Allowed: true, Reason: "admin role"}
		}
		if !store.CanAccessService(user, service) {
			return AccessDecision{Allowed: false, Reason: "service access denied"}
		}
	}

	rules := GetUserAccessPolicies(user)
	serviceRules := make([]AccessPolicyRule, 0)
	for _, rule := range rules {
		if rule.Service == service {
			serviceRules = append(serviceRules, rule)
		}
	}
	if len(serviceRules) == 0 {
		return AccessDecision{Allowed: true, Reason: "service allowlist matched"}
	}

	for _, rule := range serviceRules {
		if !strings.HasPrefix(path, rule.PathPrefix) {
			continue
		}
		if len(rule.Methods) > 0 && !slices.Contains(rule.Methods, method) {
			continue
		}
		if rule.RequireSessionMFA && authSource != "session" {
			return AccessDecision{Allowed: false, Reason: "route requires MFA-backed browser session", PolicyID: rule.ID}
		}
		return AccessDecision{Allowed: true, Reason: "route policy matched", PolicyID: rule.ID}
	}
	return AccessDecision{Allowed: false, Reason: "no route policy matched"}
}

func saveAccessPoliciesLocked() error {
	users := make(map[string][]AccessPolicyRule, len(accessPolicyData))
	for user, rules := range accessPolicyData {
		users[user] = rules
	}
	raw, err := json.MarshalIndent(accessPolicyFile{Users: users}, "", "  ")
	if err != nil {
		return err
	}
	path := accessPoliciesFilePath()
	if err := ensureParentDir(path); err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0600)
}

func normalizeAccessPolicyRules(rules []AccessPolicyRule) []AccessPolicyRule {
	out := make([]AccessPolicyRule, 0, len(rules))
	for _, rule := range rules {
		rule.Service = NormalizeAllowedServiceName(rule.Service)
		rule.PathPrefix = normalizePolicyPath(rule.PathPrefix)
		rule.Description = strings.TrimSpace(rule.Description)
		if rule.Service == "" {
			continue
		}
		if rule.ID == "" {
			rule.ID = randomHex(8)
		}
		methods := make([]string, 0, len(rule.Methods))
		seenMethods := map[string]struct{}{}
		for _, method := range rule.Methods {
			method = strings.ToUpper(strings.TrimSpace(method))
			if method == "" {
				continue
			}
			if _, ok := seenMethods[method]; ok {
				continue
			}
			seenMethods[method] = struct{}{}
			methods = append(methods, method)
		}
		sort.Strings(methods)
		rule.Methods = methods
		out = append(out, rule)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Service == out[j].Service {
			if out[i].PathPrefix == out[j].PathPrefix {
				return out[i].ID < out[j].ID
			}
			return out[i].PathPrefix < out[j].PathPrefix
		}
		return out[i].Service < out[j].Service
	})
	return out
}

func normalizePolicyPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" || path == "*" {
		return "/"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return path
}
