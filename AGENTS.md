# Apig0 Security-Readiness Agent Instructions

## Repository Purpose

Apig0 is a Go API gateway. It provides authenticated access to configured upstream services, admin controls, monitoring endpoints, and local setup flows.

## Security Operating Rules

- Preserve existing functionality and project structure.
- Keep changes small, reviewable, and compatible with the Go gateway architecture.
- Treat all security-readiness work as internal validation and audit-preparation evidence.
- Keep all generated content professional, factual, and audit-friendly.
- Do not invent scan results, screenshots, attestations, findings, approvals, or external validation.
- Mark missing, skipped, or unvalidated evidence as `Pending` or `Skipped` with a reason.

## Scoped Pentesting Rules

- Read `security/scope.md` and `security/rules-of-engagement.md` before any pentesting-style validation.
- Only test this repository, local Docker containers, localhost targets, CI test environments, staging targets explicitly listed in `security/scope.md`, or systems with written authorization.
- Refuse public target scanning, third-party testing, brute force against real accounts, destructive exploitation, persistence, credential theft, stealth, malware, backdoors, logging bypass, data exfiltration, and secret dumping.
- Record every command, target, timestamp, tool, result, and evidence path under `security/evidence/` or `security/reports/`.

## Testing Requirements

- Run relevant tests after security-impacting patches.
- For Go changes, prefer `go test ./...` and `go vet ./...`.
- Run available baseline checks such as gitleaks, semgrep, trivy, syft, grype or trivy vulnerability matching, and govulncheck when installed.
- If a required tool is unavailable, continue partial execution when safe and document the gap in `security/reports/tooling-gaps.md`.
- Keep tests deterministic and focused on the changed behavior.

## Evidence Requirements

- Security evidence lives under `security/`.
- Preserve evidence traceability to the command, tool, date, environment, operator, branch, commit, and output file.
- Store command logs under `security/evidence/commands.log`.
- Store test results under `security/evidence/test-results.md`.
- Store scan outputs under `security/evidence/scans/`.
- Store SBOM outputs under `security/evidence/sbom/`.
- Store patch notes under `security/evidence/patches/`.
- Imported artifacts from external validation environments must disclose origin, tooling attribution, validation status, and whether they came from `apig0-security-lab`.

## Compliance-Readiness Language

Allowed language:

- security readiness
- verified-ready internal
- SOC 2 readiness aligned
- ISO/IEC 27001 readiness aligned
- audit-preparation evidence
- internal validation
- OWASP-aligned
- compliance mapping
- security assessment

Forbidden language unless independently verified evidence is added by the project owner:

- Any statement that claims formal certification.
- Any statement that claims full compliance or legal compliance.
- Any statement that claims third-party auditor approval.
- Any statement that guarantees security.
- Any statement that claims the repository is free of vulnerabilities.

## Prohibited Actions

- Do not vendor external security platforms or scanning frameworks into this repository.
- Do not add unsupported external security platforms, setup steps, dependencies, docs, or workflows.
- Do not run Pentest-Swarm-AI-style testing unless targets are explicitly authorized in `security/scope.md`.
- Do not run destructive commands, wipe evidence, or revert user changes unless explicitly requested.
- Do not commit generated secrets, service tokens, TOTP secrets, passwords, private keys, or real customer data.

## Patching Rules

- Link security-relevant code changes to findings, remediation entries, tests, and evidence where relevant.
- Document every vulnerability identified, fixed, accepted, or retested in `security/reports/findings.md`.
- Update `security/reports/remediation-plan.md` and `security/reports/risk-register.md` for security-impacting patches or accepted risks.
- Prefer safe validation, input normalization, least privilege, secure defaults, and explicit authorization boundaries.
- If a finding cannot be patched safely, document it as `Open` or `Needs Manual Review`.

## Reporting Rules

Every security-readiness run must document:

- date/time
- branch
- commit hash
- commands executed
- tools used
- results
- files changed
- vulnerabilities found
- vulnerabilities fixed
- remaining risks
- failed checks
- skipped checks
- missing tools
- readiness status

All findings must include:

- severity
- status
- evidence
- affected file/path
- impact
- exploitability
- recommended fix
- patch status
- verification
- residual risk

Use severities: `Critical`, `High`, `Medium`, `Low`, `Informational`.

Use statuses: `Open`, `Patched`, `Accepted Risk`, `False Positive`, `Needs Manual Review`.

## Definition Of Done

- `AGENTS.md` exists and reflects this security-readiness workflow.
- `.codex/agents/` contains `security-auditor.toml`, `penetration-tester.toml`, `compliance-auditor.toml`, and `test-automator.toml`.
- `security/scope.md`, `security/rules-of-engagement.md`, and `security/readiness-stamp.md` exist.
- Security evidence and reports are updated under `security/evidence/` and `security/reports/`.
- Relevant tests and scans ran, or skipped checks are documented with reasons.
- No unresolved high or critical findings remain before using `VERIFIED-READY INTERNAL`.
- `security/readiness-stamp.md` remains honest and evidence-based.
- No certification, formal compliance, legal compliance, or third-party audit approval is claimed.
