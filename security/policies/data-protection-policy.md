# Data Protection Policy

## Purpose

Define lightweight data-protection expectations for Apig0 secrets, tokens, logs, and validation evidence.

## Policy

- Treat gateway tokens, upstream credentials, TOTP secrets, master passwords, and imported scan artifacts as sensitive.
- Store raw gateway tokens only in one-time delivery paths or operator-controlled output where the application requires it.
- Prefer hashed token storage and encrypted service-secret storage where supported.
- Do not commit production credentials, unredacted secrets, or private customer data.
- Redact sensitive values from reports, screenshots, logs, and imported evidence.
- Keep data-protection claims limited to documented controls and available evidence.

## Evidence

- Secret storage code: `config/service_secrets.go`, `config/vault.go`
- Token storage code: `config/apitokens.go`
- Evidence index: `security/evidence/evidence-index.md`
- Secret scanning outputs: `security/scans/`
