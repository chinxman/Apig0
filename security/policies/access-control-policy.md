# Access Control Policy

## Purpose

Define practical access-control expectations for Apig0 development and operation.

## Policy

- Administrative access should be limited to users who need to manage gateway configuration, users, services, secrets, tokens, rate limits, and audit data.
- Browser login should use password and TOTP where enabled by the application flow.
- Gateway tokens should be scoped to the minimum required services, models, providers, and expiration period.
- Raw gateway tokens should be shown only once and handled as sensitive credentials.
- Admin accounts should be reviewed before demos, deployments, or assessment activities.
- Access changes should be traceable to an issue, pull request, finding, or operator action when practical.

## Evidence

- Access-control evidence location: `security/evidence/`
- Findings location: `security/reports/findings.md`
- Remediation tracking: `security/reports/remediation-plan.md`

## Review Cadence

Review this policy before major releases and after security-impacting changes.
