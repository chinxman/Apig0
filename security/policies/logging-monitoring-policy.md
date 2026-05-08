# Logging and Monitoring Policy

## Purpose

Define practical logging and monitoring expectations for Apig0 security readiness.

## Policy

- Logs should support investigation of authentication, token, admin, routing, proxy, rate-limit, and error events.
- Logs should avoid raw gateway tokens, upstream service secrets, passwords, TOTP secrets, and master passwords.
- Security-relevant logs collected as evidence should be stored under `security/evidence/logs/` only when needed.
- Metrics and observability endpoints should require appropriate access controls in non-local environments.
- Security validation should review whether errors reveal sensitive implementation details.

## Evidence

- Runtime logs: `security/evidence/logs/`
- Monitoring code: `middleware/monitor.go`
- Metrics code: `middleware/prometheus.go`
- Audit code: `config/audit.go`

## Review Triggers

Review this policy when logging fields, monitoring endpoints, metrics authentication, or audit behavior changes.
