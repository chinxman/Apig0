# Security Evidence

This directory stores internal security-readiness evidence for Apig0.

Evidence must be traceable to:

- command
- tool
- date/time
- environment
- operator
- branch
- commit
- output file
- validation status

## Directory Layout

| Path | Purpose |
| --- | --- |
| `commands.log` | Command and tool execution log |
| `test-results.md` | Test and scan result summary |
| `scans/` | SAST, secret, dependency, vulnerability, and API scan outputs |
| `sbom/` | SBOM files |
| `patches/` | Patch evidence and remediation notes |

## Evidence Rules

- Mark missing or skipped evidence as `Pending` or `Skipped`.
- Do not invent scan output, screenshots, findings, or attestations.
- Redact accidental secrets from human-readable reports.
- Keep raw tool output where safe and appropriate.
