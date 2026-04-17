# TODO

Local note:
- This planning file is for local working context and is not intended to be pushed to GitHub.
- GitHub is intentionally out of scope for now. Work stays local in this repository until explicitly revisited.
- Code edits happen here. Runtime and server verification are done by the user on the real server environment.

---

## Done (this session)

### Setup Wizard — Fixed & Working
- [x] **Setup overlay flash/stuck bug** — removed hardcoded `visible` class from HTML; `init()` now explicitly determines which screen to show (setup, login, bootstrap, or app)
- [x] **Service row remove button** — broken `\\'` quote escaping in `addSetupService()` caused JS syntax error; replaced with standalone `removeServiceRow()` function
- [x] **`init()` error handling** — catch-all no longer shows setup wizard on network errors; shows login screen instead
- [x] **Bootstrap-only mode** — when setup is complete but admin is missing, shows a simple "Create Admin" form instead of the full setup wizard; uses `/api/setup/bootstrap-admin` endpoint
- [x] **`BootstrapRequired` stale value** — moved computation in `runtime_status.go` to run after `SetupRequired` overrides so downstream consumers never see inconsistent state
- [x] **Storage mode indicator** — Users page now shows System Storage card with: Storage Mode (Temporary/Persistent), Secrets Backend, Users Backend, Service Count
- [x] **Upgrade to Persistent** — admin can upgrade from temporary to persistent mode from the Users page; picks TOTP vault backend (file, hashicorp, aws, gcp, azure) and service secret storage (file, encrypted file); migrates all in-memory state to disk
- [x] **Backend: `POST /api/admin/settings/storage`** — new admin endpoint handles the upgrade, snapshots TOTP secrets before re-init, flushes users/services/rate-limits to JSON files
- [x] **`hideAllOverlays()` helper** — all screen transitions now go through one function to prevent overlay stacking bugs

### Files Modified
- `webui.html` — setup overlay, bootstrap overlay, upgrade modal, init flow, storage indicator
- `auth/setup.go` — `UpgradeStorageHandler`, fixed secret migration
- `config/setup_runtime.go` — `UpgradeToPersistent()` function
- `config/runtime_status.go` — fixed `BootstrapRequired` computation order
- `main.go` — registered `POST /api/admin/settings/storage` route

---

## Next Up

### 1. Remove "Hard Reset Setup" from Login Page (HIGH)
The login screen has a "Hard Reset Setup" button (`resetSetupFlow()`) that instantly wipes ALL data — users, services, secrets, everything. This is exposed on the **unauthenticated** login page. One misclick or unauthorized person destroys the config.

**Action:** Remove the button from `#login-overlay`. Keep reset available only behind admin auth.

**Later:** Replace with a secure recovery mechanism:
- Email-based identity verification
- Secure reset link (time-limited, single-use)
- Admin-only flow, not public-facing
- Confirmation step with explicit "type DELETE to confirm" pattern

### 2. Per-API Rate Limiting (MEDIUM)
Currently rate limits are global per-user. Add per-service rate limiting so each API endpoint can have its own limits.

### 3. Admin-Managed Service Configuration (MEDIUM)
After initial setup, there's no way to add/edit/remove services from the admin panel. Admin should be able to:
- Add new API services
- Edit existing service URLs, auth types, secrets
- Enable/disable services
- Remove services

### 4. Service Secrets Management API (MEDIUM)
`SetServiceSecret()`, `DeleteServiceSecret()`, `GetServiceSecret()` exist in the backend but aren't exposed via API. Admin needs endpoints to manage service credentials after setup.

### 5. Test Console Review (LOW)
Monitor page has a "Test Console" that sends requests to endpoints. Review whether this should stay, be enhanced, or be removed.

### 6. sest endpoint by user
Make it so admin has all endpoints setup basically adding a seccion where the admin controls not just rate imiting but the api key and endppoints per user.

---

## Architecture Notes

### Storage Modes
- **Temporary** — everything in-memory + temp files. Restart = back to setup wizard. Good for testing.
- **Persistent** — `apig0-setup.json` + `users.json` + `services.json` + `ratelimits.json` + vault backend for TOTP. Survives restarts.
- **Upgrade path** — admin can upgrade temporary → persistent from the Users page without re-running setup.

### Auth Flow
1. Setup wizard → first admin + TOTP QR
2. Login: password → challenge token → TOTP code → session cookie
3. Session: 8h default, server-side map, HttpOnly + SameSite=Strict cookie
4. Admin routes: session + admin role + CSRF (double-submit cookie)

### Vault Backends
env, file, hashicorp, aws, gcp, azure, cyberark, 1password, http, exec
