# Incident Response Policy

## Purpose

Provide a lightweight incident response process for Apig0 security events.

## Incident Examples

- Exposed gateway token, service secret, or master password
- Unauthorized admin access
- Bypass of TOTP, token scope, route policy, or rate limit controls
- Sensitive data in logs or scan artifacts
- Confirmed dependency vulnerability affecting Apig0

## Response Process

1. Triage the report and preserve relevant logs or evidence.
2. Assess affected components, endpoints, users, services, and secrets.
3. Contain the issue by revoking tokens, rotating secrets, disabling affected services, or limiting access.
4. Record the issue in `security/reports/findings.md` if validated.
5. Track corrective actions in `security/reports/remediation-plan.md`.
6. Retest the fix and attach evidence.
7. Document lessons learned when the issue affects architecture or process.

## Evidence

- Logs: `security/evidence/logs/`
- Evidence index: `security/evidence/evidence-index.md`
- Findings: `security/reports/findings.md`
- Risk register: `security/reports/risk-register.md`
