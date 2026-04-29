package config

import "os"

func ReloadRuntime(serviceSecretOverrides map[string]string, masterPassword string) error {
	InitSecrets()
	LoadRateLimits()
	LoadServices()
	if err := LoadAPITokens(); err != nil {
		return err
	}
	if masterPassword == "" {
		masterPassword = os.Getenv("APIG0_SERVICE_MASTER_PASSWORD")
	}
	if err := LoadPendingAPITokenDeliveries(masterPassword); err != nil {
		return err
	}
	if err := LoadAccessPolicies(); err != nil {
		return err
	}
	if err := LoadServiceSecretMetadata(); err != nil {
		return err
	}

	setup := CurrentSetupConfig()
	return ConfigureServiceSecrets(setup.ServiceSecrets, serviceSecretOverrides, masterPassword)
}
