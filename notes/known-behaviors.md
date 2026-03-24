# Known Behaviors

## Environment Variable Config — Planned for Removal

**What it is:**
All gateway settings (`VAULT_TYPE`, `VAULT_ADDRESS`, `APIG0_USERS`, `APIG0_SESSION_TTL`, etc.)
can still be set via environment variables. They override `apig0.yaml` when present.

**Why it still exists:**
Kept intentionally for dev/testing — lets you quickly switch vault backends or override
settings without touching the config file (e.g. `export VAULT_TYPE=env` for a local test run).

**Planned:**
Remove env var support for non-secret config once `apig0.yaml` is the established
single source of truth. Secrets (tokens, passwords, TOTP keys) will remain in the
vault backend and never in the file.

**Relevant code:**
`config/appconfig.go` — `LoadAppConfig()`, `setDefault()`

---


## TOTP Reuse Block After Logout

**What happens:**
If a user logs in with a TOTP code, logs out, and immediately tries to log in again
using the same TOTP code — still within the 30-second window shown on their authenticator
app — the gateway will reject it until the code rotates.

**Why:**
The anti-replay cache marks every successfully used code as consumed for 90 seconds
(the full ±1-period skew window). This is intentional — it prevents replay attacks
where someone intercepts a valid code and reuses it.

**Impact:**
User has to wait for their authenticator app to cycle to the next 30-second code
before they can log back in. No refresh needed, just wait for the new code.

**Status:**
Solid security feature for now. If it causes UX friction later (e.g. power users
logging in and out rapidly), the fix would be to reduce the replay window to exactly
one period (30s) and drop the ±1 skew — accepting the trade-off of stricter clock sync.

**Relevant code:**
`auth/totp.go` — `ValidateTOTP()`, `usedCodes` map, 90s expiry
