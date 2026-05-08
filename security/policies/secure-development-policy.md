# Secure Development Policy

## Purpose

Define lightweight secure SDLC expectations for Apig0 as an open-source student capstone and small-team API Gateway project.

## Policy

- Preserve existing functionality and route behavior unless a change is explicitly scoped.
- Run Go tests before merging security-sensitive changes.
- Use security-readiness checks when changes affect authentication, TOTP, token handling, routing, proxy behavior, middleware, CORS, rate limiting, secrets, logging, or observability.
- Document security-impacting architecture changes in README, mappings, reports, or policies as appropriate.
- Track validated vulnerabilities in `security/reports/findings.md`.
- Track fixes and retest status in `security/reports/remediation-plan.md`.
- Do not add heavy third-party scanning platforms to this product repository.
- Keep generated evidence professional, factual, and marked `Pending` until validated.

## Evidence

- CI workflow: `.github/workflows/security-readiness.yml`
- Local scripts: `security/scripts/`
- Scan outputs: `security/scans/`
