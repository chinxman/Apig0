package config

import (
	"encoding/json"
	"log"
	"os"
	"sync"
)

type RateLimitRule struct {
	RequestsPerMinute int `json:"requests_per_minute"`
	Burst             int `json:"burst"`
}

type RateLimitSettings struct {
	Default RateLimitRule            `json:"default"`
	Users   map[string]RateLimitRule `json:"users"`
}

var (
	rlMu   sync.RWMutex
	rlData = RateLimitSettings{
		Default: RateLimitRule{RequestsPerMinute: 60, Burst: 10},
		Users:   make(map[string]RateLimitRule),
	}
	rlPath = "ratelimits.json"
)

func LoadRateLimits() RateLimitSettings {
	if path := os.Getenv("APIG0_RATELIMITS_PATH"); path != "" {
		rlPath = path
	}
	raw, err := os.ReadFile(rlPath)
	if err != nil {
		return rlData
	}
	rlMu.Lock()
	defer rlMu.Unlock()
	if err := json.Unmarshal(raw, &rlData); err != nil {
		log.Printf("[ratelimits] failed to parse ratelimits.json: %v", err)
	}
	if rlData.Users == nil {
		rlData.Users = make(map[string]RateLimitRule)
	}
	log.Printf("[ratelimits] loaded (default: %d rpm)", rlData.Default.RequestsPerMinute)
	return rlData
}

func GetRateLimits() RateLimitSettings {
	rlMu.RLock()
	defer rlMu.RUnlock()
	return rlData
}

func SaveRateLimits(s RateLimitSettings) error {
	rlMu.Lock()
	defer rlMu.Unlock()
	if path := os.Getenv("APIG0_RATELIMITS_PATH"); path != "" {
		rlPath = path
	}
	if s.Users == nil {
		s.Users = make(map[string]RateLimitRule)
	}
	rlData = s
	raw, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(rlPath, raw, 0644)
}
