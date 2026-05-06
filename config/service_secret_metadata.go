package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type ServiceSecretMetadata struct {
	Service        string    `json:"service"`
	LastRotatedAt  time.Time `json:"last_rotated_at,omitempty"`
	LastTestedAt   time.Time `json:"last_tested_at,omitempty"`
	LastTestStatus int       `json:"last_test_status,omitempty"`
	ExpiresAt      time.Time `json:"expires_at,omitempty"`
	Notes          string    `json:"notes,omitempty"`
}

type serviceSecretMetadataFile struct {
	Metadata map[string]ServiceSecretMetadata `json:"metadata"`
}

var (
	serviceSecretMetaMu   sync.RWMutex
	serviceSecretMetaData = map[string]ServiceSecretMetadata{}
	serviceSecretMetaPath = filepath.Join(os.TempDir(), "apig0-service-secret-metadata-"+randomSuffix()+".json")
)

func serviceSecretMetadataPath() string {
	if path := strings.TrimSpace(os.Getenv("APIG0_SERVICE_SECRET_METADATA_PATH")); path != "" {
		return path
	}
	if CurrentSetupConfig().Mode == SetupModePersistent {
		return "service-secret-metadata.json"
	}
	return serviceSecretMetaPath
}

func LoadServiceSecretMetadata() error {
	serviceSecretMetaMu.Lock()
	defer serviceSecretMetaMu.Unlock()

	serviceSecretMetaData = map[string]ServiceSecretMetadata{}
	raw, err := os.ReadFile(serviceSecretMetadataPath())
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var file serviceSecretMetadataFile
	if err := json.Unmarshal(raw, &file); err != nil {
		return err
	}
	for service, meta := range file.Metadata {
		service = NormalizeAllowedServiceName(service)
		if service == "" {
			continue
		}
		meta.Service = service
		serviceSecretMetaData[service] = meta
	}
	return nil
}

func ListServiceSecretMetadata() map[string]ServiceSecretMetadata {
	serviceSecretMetaMu.RLock()
	defer serviceSecretMetaMu.RUnlock()

	out := make(map[string]ServiceSecretMetadata, len(serviceSecretMetaData))
	for service, meta := range serviceSecretMetaData {
		out[service] = meta
	}
	return out
}

func UpdateServiceSecretMetadata(service string, meta ServiceSecretMetadata) error {
	serviceSecretMetaMu.Lock()
	defer serviceSecretMetaMu.Unlock()

	service = NormalizeAllowedServiceName(service)
	if service == "" {
		return os.ErrInvalid
	}
	meta.Service = service
	meta.Notes = strings.TrimSpace(meta.Notes)
	serviceSecretMetaData[service] = meta
	return saveServiceSecretMetadataLocked()
}

func NoteServiceSecretRotated(service string, expiresAt time.Time, notes string) error {
	meta := GetServiceSecretMetadata(service)
	meta.ExpiresAt = expiresAt.UTC()
	meta.Notes = strings.TrimSpace(notes)
	meta.LastRotatedAt = time.Now().UTC()
	return UpdateServiceSecretMetadata(service, meta)
}

func NoteServiceSecretTest(service string, status int) error {
	meta := GetServiceSecretMetadata(service)
	meta.LastTestedAt = time.Now().UTC()
	meta.LastTestStatus = status
	return UpdateServiceSecretMetadata(service, meta)
}

func DeleteServiceSecretMetadata(service string) error {
	serviceSecretMetaMu.Lock()
	defer serviceSecretMetaMu.Unlock()

	delete(serviceSecretMetaData, NormalizeAllowedServiceName(service))
	return saveServiceSecretMetadataLocked()
}

func GetServiceSecretMetadata(service string) ServiceSecretMetadata {
	serviceSecretMetaMu.RLock()
	defer serviceSecretMetaMu.RUnlock()
	return serviceSecretMetaData[NormalizeAllowedServiceName(service)]
}

func saveServiceSecretMetadataLocked() error {
	raw, err := json.MarshalIndent(serviceSecretMetadataFile{Metadata: serviceSecretMetaData}, "", "  ")
	if err != nil {
		return err
	}
	path := serviceSecretMetadataPath()
	if err := ensureParentDir(path); err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0600)
}
