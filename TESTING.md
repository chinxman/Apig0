# Testing And Recovery

`VAULT_TYPE=env` is temporary mode.
Passwords loaded from `APIG0_PASSWORD_<USER>` and TOTP secrets loaded from `APIG0_TOTP_SECRET_<USER>` only survive if those env vars are present again after restart. Web-created TOTP changes are not durable in this mode.

`VAULT_TYPE=file` is the local stationary mode.
Users stay in `users.json` and TOTP secrets stay in `totp-secrets.json` or `VAULT_FILE_PATH`. This is the recommended local setup for testing restart and recovery flows without an external vault.

The web UI now exposes:

- `GET /api/setup/status` to show whether the instance is temporary or persistent.
- `POST /api/setup/bootstrap-admin` to create the first admin account when no admin exists yet.

If the gateway restarts and no admin is present, open the web UI and use the initial admin setup flow. If you want the same users to remain valid after restart, switch to `VAULT_TYPE=file` or a real vault backend.
