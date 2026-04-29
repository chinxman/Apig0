# Apig0

Apig0 is a Go-based internal API gateway with a built-in web admin UI.

## Route Map

- `GET /` serves the web UI shell.
- `GET /healthz` returns a basic health response.
- `GET /metrics` exposes Prometheus-style gateway metrics.
- `GET /api/setup/status` reports setup and storage mode state.
- `POST /api/setup/complete` completes first-run setup.
- `POST /api/setup/bootstrap-admin` creates an admin when setup exists but no admin remains.
- `POST /auth/login`, `POST /auth/verify`, `POST /auth/logout` handle browser auth.
- `GET /api/user/info` returns current user/session or token context.
- `GET /api/admin/*` and `POST/PUT/DELETE /api/admin/*` back the admin UI.
- `ANY /{service}/...` proxies traffic to configured upstream services after auth and policy checks.

## Admin CLI

`apig0` now includes a built-in admin CLI alongside the web server.

- `apig0` with no arguments starts the gateway server in the background.
- `apig0 start` explicitly starts the gateway server in the background.
- `apig0 stop` stops the managed background server with `SIGTERM`.
- `apig0 restart` restarts the managed background server.
- `apig0 serve` explicitly starts the gateway server.
- `apig0 logs [-n N] [-f]` shows the background server log.
  Without `-f` it prints a snapshot and exits. With `-f` it stays in the foreground and follows the log live.
- `apig0 monitor [-n N] [-f] [--service name] [--errors]` shows structured request activity from the current background run.
  Without `-f` it prints a snapshot and exits. With `-f` it stays in the foreground and streams live request events.
- `apig0 status` prints runtime, setup, and storage state.
- `apig0 setup status` prints the same setup/runtime state.
- `apig0 setup reset --force` wipes setup and persistent gateway state, then reloads runtime.
- `apig0 setup bootstrap-admin --username <name> --password <pass>` creates an admin only when setup is complete and no admin currently exists.
- `apig0 users list`
- `apig0 users add --username <name> --password <pass> [--role user|admin] [--services svc1,svc2]`
- `apig0 users delete --username <name>`
- `apig0 services list`
- `apig0 services add --name <svc> --url <base> [--auth-type ...] [--header ...] [--basic-username ...] [--timeout-ms ...] [--retry-count ...] [--secret ...]`
- `apig0 services delete --name <svc>`
- `apig0 tokens list`
- `apig0 tokens create --user <name> [--name ...] [--services svc1,svc2] [--expires-at RFC3339]`
- `apig0 tokens revoke --id <token-id>`

### CLI Notes

- The CLI works against the same local runtime and storage layer as the web UI. It does not call the HTTP admin API.
- Background server logs are written to a temporary runtime log by default: `${TMPDIR:-/tmp}/apig0-runtime.log`. Set `APIG0_LOG_PATH` to change that path.
- `go run main.go` now behaves like `go run main.go start`, so the shell returns immediately after printing the startup URL and log path.
- Use `go run main.go serve` when you want the gateway attached to the current terminal.
- `start` rewrites the runtime log for each new background launch so `logs` behaves like a live monitor for the current run instead of a cumulative archive.
- `stop` targets the managed background PID written by `start`. Use `stop --force` if graceful shutdown does not complete.
- Use `go run main.go logs -f` to watch startup, gin request logs, and runtime logging from the current background server run.
- `monitor` is separate from `logs`. It streams structured request events captured by the gateway middleware rather than raw log lines.
- `start` also resets the structured monitor stream for the new run, so `monitor` shows only current-session traffic by default.
- `logs` and `monitor` both default to snapshot mode. Add `-f` when you want an attached live feed.
- `users add` prints the generated `otpauth://` URL so the new user's TOTP seed can be enrolled.
- `services add --secret ...` stores upstream credentials in the configured service-secret backend.
- `tokens create` prints the raw token once. Store it securely when it is created.
- `setup reset --force` is destructive and removes the saved setup/runtime files used by persistent mode.

## Package Layout

- [`main.go`](main.go): startup, route wiring, TLS mode, static asset serving.
- [`auth/`](auth): browser session auth, token auth, admin/setup handlers, TOTP flows.
- [`config/`](config): runtime config, services, users, tokens, policies, audit, storage backends.
- [`middleware/`](middleware): CORS, CSRF, rate limiting, monitoring, Prometheus output.
- [`proxy/`](proxy): reverse proxy behavior, upstream auth injection, timeout/retry handling.
- [`cli/`](cli): built-in admin CLI for local operator management.
- [`features/`](features): implementation notes for major passes.

## UI Layout

- [`webui.html`](webui.html): HTML shell only.
- [`static/css/webui.css`](static/css/webui.css): UI styling.
- [`static/vendor/qrcode.min.js`](static/vendor/qrcode.min.js): local QR dependency.
- [`static/js/app-core.js`](static/js/app-core.js): shared app state, DOM helpers, API helpers.
- [`static/js/auth.js`](static/js/auth.js): login, bootstrap admin, session boot, QR modal.
- Portal is now a service-aware command generator on the main page. Users select an allowed service, fill method/path/token/body, and copy ready-to-run snippets into their own terminal or editor.
- The Portal generator currently emits `curl`, `bash`, `python`, and `javascript` snippets against the gateway path `/{service}/...`.
- Terminal-oriented snippets are token-based by design. Paste a scoped gateway token into the Portal generator or leave the placeholder in place until a token is issued.
- [`static/js/setup.js`](static/js/setup.js): setup mode selection, storage upgrade, reset flow.
- [`static/js/monitor.js`](static/js/monitor.js): SSE monitor, request log, audit panel, test console.
- [`static/js/admin-services.js`](static/js/admin-services.js): service CRUD and secret metadata UI.
- [`static/js/admin-users.js`](static/js/admin-users.js): user CRUD, access controls, policy editing.
- [`static/js/admin-tokens.js`](static/js/admin-tokens.js): gateway token management.
- [`static/js/admin-ratelimits.js`](static/js/admin-ratelimits.js): rate limit editor UI.
- [`static/js/navigation.js`](static/js/navigation.js): page switching and per-page data loading.
- [`static/js/bootstrap.js`](static/js/bootstrap.js): delegated event wiring and app startup.

## Storage Modes

- Temporary mode keeps the active setup only for the running gateway process. Browser refresh does not reset it; restarting the gateway returns to first-run setup.
- Temporary mode always keeps service secrets in memory and uses the env-backed TOTP secret path by default.
- Persistent mode writes gateway state to local files or supported secret backends so restarts keep users and configuration.
- Persistent service secret storage is limited to two modes:
  - `Non-Encrypted File`
  - `Encrypted File`
- Persistent setup normalizes service-secret storage to file-backed modes only. The in-memory `memory` mode is reserved for temporary runtime.
- Setup/storage details are also described in [`TESTING.md`](TESTING.md) and [`apig0.yaml`](apig0.yaml).

## Vault Provider Setup

Persistent setup and the storage-upgrade flow support these primary TOTP secret backends in the web UI:

- `Local File`
- `Hashicorp Vault`
- `AWS`
- `GCP`
- `Azure`
- `1Password`

Advanced providers are still available behind the UI's `Advanced vault providers` section:

- `CyberArk`
- `HTTP`
- `Exec`

### Provider Notes

- `Local File` stores TOTP secrets in `totp-secrets.json` by default.
- `Hashicorp Vault` requires a vault address and engine.
- `AWS` uses the `aws` CLI backend. The setup UI can persist region/profile and optional key material for runtime use.
- `GCP` uses the `gcloud` CLI backend. `GCP_PROJECT` is required. A credentials file path can be stored through setup.
- `Azure` uses the `az` CLI backend. `AZURE_VAULT_NAME` is required. Tenant/client fields can also be stored through setup.
- `1Password` uses the `op` CLI backend. The setup UI can persist `OP_VAULT` and an optional `OP_SERVICE_ACCOUNT_TOKEN`.
- `CyberArk`, `HTTP`, and `Exec` are implemented as specialist or advanced backends and are not part of the primary setup path.

### Operational Expectation

- Provider-specific setup values entered in the web UI are persisted in setup state and re-applied on runtime reload.
- CLI-backed providers still depend on the host environment being capable of running and authenticating those CLIs.
- If a provider cannot initialize or pass its health check, the gateway logs the failure and falls back to the env backend for TOTP secrets.
