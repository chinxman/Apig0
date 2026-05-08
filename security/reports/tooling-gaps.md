# Tooling Gaps

## Metadata

| Field | Value |
| --- | --- |
| Date/time | 2026-05-08T21:45:00Z |
| Branch | security-readiness-evidence-pass |
| Commit hash | 27b82f0c88ef1271148755223bfb883a19d87328 |
| Operator | Codex |

## Current Gap Status

| Tool or area | Status | Reason | Recommended action |
| --- | --- | --- | --- |
| OWASP ZAP baseline | Skipped | Compatible local ZAP baseline runtime or Docker is not available in this environment | Install a compatible runtime or keep relying on the current scoped curl/nmap/nuclei evidence with explicit acknowledgment |
| GitHub Actions workflow execution | Pending | Workflow file is staged, but this pre-commit review does not run GitHub-hosted CI | Run the workflow on a pull request or via `workflow_dispatch` after commit |

## Notes

- Current readiness evidence includes Semgrep, Trivy, Syft, and Grype artifacts in the staged package.
- Missing local reruns of those tools during this pre-commit review do not invalidate the already staged current evidence files, but future refreshes should preserve the same traceability.
- Missing or skipped tooling remains visible rather than being treated as implicitly passed.
