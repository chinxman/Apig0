package config

import "os"

func ReloadRuntime(serviceSecretOverrides map[string]string, masterPassword string) error {
	InitSecrets()
	LoadRateLimits()
	LoadServices()

	setup := CurrentSetupConfig()
	if masterPassword == "" {
		masterPassword = os.Getenv("APIG0_SERVICE_MASTER_PASSWORD")
	}
	return ConfigureServiceSecrets(setup.ServiceSecrets, serviceSecretOverrides, masterPassword)
}
