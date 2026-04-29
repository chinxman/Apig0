package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const maxRecentAuditEvents = 500

type AuditEvent struct {
	ID         string `json:"id"`
	Timestamp  int64  `json:"ts"`
	User       string `json:"user,omitempty"`
	AuthSource string `json:"auth_source,omitempty"`
	Service    string `json:"service,omitempty"`
	Method     string `json:"method,omitempty"`
	Path       string `json:"path,omitempty"`
	Decision   string `json:"decision"`
	Reason     string `json:"reason,omitempty"`
	PolicyID   string `json:"policy_id,omitempty"`
	TokenID    string `json:"token_id,omitempty"`
	ClientIP   string `json:"client_ip,omitempty"`
	Status     int    `json:"status,omitempty"`
	Upstream   string `json:"upstream,omitempty"`
}

type AuditCounters struct {
	Allow int64 `json:"allow"`
	Deny  int64 `json:"deny"`
	Other int64 `json:"other"`
}

var (
	auditMu      sync.RWMutex
	auditEvents  []AuditEvent
	auditCounts  AuditCounters
	auditLogPath = filepath.Join(os.TempDir(), "apig0-audit-"+randomSuffix()+".log")
)

func auditFilePath() string {
	if path := strings.TrimSpace(os.Getenv("APIG0_AUDIT_LOG_PATH")); path != "" {
		return path
	}
	if CurrentSetupConfig().Mode == SetupModePersistent {
		return "audit.log"
	}
	return auditLogPath
}

func RecordAuditEvent(evt AuditEvent) {
	if evt.ID == "" {
		evt.ID = randomHex(8)
	}
	if evt.Timestamp == 0 {
		evt.Timestamp = time.Now().UnixMilli()
	}

	auditMu.Lock()
	switch evt.Decision {
	case "allow":
		auditCounts.Allow++
	case "deny":
		auditCounts.Deny++
	default:
		auditCounts.Other++
	}
	auditEvents = append(auditEvents, evt)
	if len(auditEvents) > maxRecentAuditEvents {
		auditEvents = append([]AuditEvent(nil), auditEvents[len(auditEvents)-maxRecentAuditEvents:]...)
	}
	auditMu.Unlock()

	raw, err := json.Marshal(evt)
	if err != nil {
		return
	}
	path := auditFilePath()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil && filepath.Dir(path) != "." {
		return
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.Write(append(raw, '\n'))
}

func ListRecentAuditEvents(limit int) []AuditEvent {
	auditMu.RLock()
	defer auditMu.RUnlock()

	if limit <= 0 || limit > len(auditEvents) {
		limit = len(auditEvents)
	}
	start := len(auditEvents) - limit
	if start < 0 {
		start = 0
	}
	out := make([]AuditEvent, limit)
	copy(out, auditEvents[start:])
	return out
}

func GetAuditCounters() AuditCounters {
	auditMu.RLock()
	defer auditMu.RUnlock()
	return auditCounts
}

func AuditLogPath() string {
	return auditFilePath()
}

func ResetAuditState() {
	auditMu.Lock()
	defer auditMu.Unlock()
	auditEvents = nil
	auditCounts = AuditCounters{}
}
