package config

import "fmt"

type RuntimeStatus struct {
	HasAdmin              bool                `json:"has_admin"`
	AdminCount            int                 `json:"admin_count"`
	BootstrapRequired     bool                `json:"bootstrap_required"`
	SetupRequired         bool                `json:"setup_required"`
	SetupMode             string              `json:"setup_mode"`
	PersistentConfigured  bool                `json:"persistent_configured"`
	Port                  string              `json:"port"`
	SecretsBackend        string              `json:"secrets_backend"`
	SecretsMode           string              `json:"secrets_mode"`
	SecretsPath           string              `json:"secrets_path,omitempty"`
	UsersBackend          string              `json:"users_backend"`
	UsersPath             string              `json:"users_path,omitempty"`
	ServiceCount          int                 `json:"service_count"`
	APITokenCount         int                 `json:"api_token_count"`
	PolicyUserCount       int                 `json:"policy_user_count"`
	ServiceSecrets        ServiceSecretConfig `json:"service_secrets"`
	AuditLogPath          string              `json:"audit_log_path,omitempty"`
	NodeMode              string              `json:"node_mode"`
	ResetBehavior         string              `json:"reset_behavior"`
	RecoveryHint          string              `json:"recovery_hint"`
	PortChangeRequiresRun bool                `json:"port_change_requires_restart"`
}

func GetRuntimeStatus() RuntimeStatus {
	setup := CurrentSetupConfig()
	store := GetUserStore()
	serviceSecrets := ServiceSecretStatus()

	status := RuntimeStatus{
		SetupRequired:         !SetupConfigured() && setup.RequiresSetup,
		SetupMode:             string(setup.Mode),
		PersistentConfigured:  SetupConfigured(),
		Port:                  setup.Port,
		SecretsBackend:        ActiveVaultName(),
		UsersBackend:          "file",
		UsersPath:             setup.UsersPath,
		ServiceCount:          len(ListServiceNames()),
		APITokenCount:         APITokenCount(),
		PolicyUserCount:       AccessPolicyUserCount(),
		ServiceSecrets:        serviceSecrets,
		AuditLogPath:          AuditLogPath(),
		NodeMode:              "single-node",
		PortChangeRequiresRun: true,
		ResetBehavior:         "Temporary mode lasts only for the running gateway process. Browser refresh keeps the current setup; restarting the gateway returns it to the setup flow.",
		RecoveryHint:          "Complete setup in persistent mode to keep users, services, rate limits, access policies, and API tokens across restart.",
	}

	if store != nil {
		status.UsersBackend = store.Backend()
		status.UsersPath = store.FilePath()
		for _, user := range store.List() {
			if user.Role == "admin" {
				status.AdminCount++
			}
		}
	}

	if status.SecretsBackend == "" {
		status.SecretsBackend = setup.UserVault.Type
	}
	if status.SecretsBackend == "" {
		status.SecretsBackend = "env"
	}

	switch status.SecretsBackend {
	case "file":
		status.SecretsMode = "persistent"
		status.SecretsPath = ActiveVaultFilePath()
		status.ResetBehavior = fmt.Sprintf("Local persistent mode is active. Users, policies, API tokens, and TOTP secrets stay available after restart while %s and %s remain intact.", status.UsersPath, status.SecretsPath)
		status.RecoveryHint = "Keep the local JSON files and your service master password safe."
	case "env":
		status.SecretsMode = "temporary"
	default:
		status.SecretsMode = "persistent"
		status.ResetBehavior = fmt.Sprintf("%s vault mode is active. Restarting the gateway keeps user secrets in the configured backend.", status.SecretsBackend)
		status.RecoveryHint = "If the server restarts, reconnect the configured vault backend and provide the service master password if local encrypted service secrets are enabled."
	}

	status.HasAdmin = status.AdminCount > 0

	// Override SetupRequired BEFORE computing BootstrapRequired so
	// downstream consumers never see a stale combination.
	if setup.Mode == SetupModePersistent {
		status.SetupRequired = !SetupConfigured()
	}
	if setup.Mode == SetupModeTemporary && !setup.RequiresSetup {
		status.SetupRequired = false
	}

	status.BootstrapRequired = status.SetupRequired || !status.HasAdmin
	return status
}
