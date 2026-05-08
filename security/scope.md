# Security Validation Scope

## Project

Project name: Apig0

## Authorization Status

Approval statement: Repository owner authorized controlled Apig0 validation for `https://192.168.12.192:8989`, `http://192.168.12.192:8989` for protocol checks, and `192.168.12.192` port `8989` only on 2026-05-07.

Owner/contact: Pending

Testing window: Pending

Emergency stop condition: Stop immediately if testing affects availability, data integrity, privacy, third-party services, production systems, or any asset not listed as authorized.

## Authorized Assets

- This repository source code and configuration.
- Localhost services started from this repository.
- Local Docker containers built from this repository when explicitly started for validation.
- CI test environments for this repository.
- `https://192.168.12.192:8989` for controlled Apig0 HTTPS validation.
- `http://192.168.12.192:8989` for controlled Apig0 HTTP validation.
- `192.168.12.192` only for controlled host/port validation related to Apig0 on port `8989`.
- Staging targets explicitly added below by the repository owner.

## Local Targets

| Target | Authorization basis | Status |
| --- | --- | --- |
| `localhost` | Local development and validation for this repository | Authorized |
| `127.0.0.1` | Local development and validation for this repository | Authorized |
| `::1` | Local development and validation for this repository | Authorized |
| Local Docker containers built from this repository | Repository-owned local validation | Authorized when started by the operator |

## Authorized Live Targets

| Target | Authorization basis | Allowed checks | Status |
| --- | --- | --- | --- |
| `https://192.168.12.192:8989` | Repository owner authorization on 2026-05-07 | Controlled HTTP checks over TLS, OWASP ZAP baseline, low-rate nuclei templates | Authorized |
| `http://192.168.12.192:8989` | Repository owner authorization on 2026-05-07 | Controlled HTTP checks, OWASP ZAP baseline, low-rate nuclei templates | Authorized |
| `192.168.12.192:8989` | Repository owner authorization on 2026-05-07 | Nmap service/version check on port `8989` only | Authorized |

## Staging Targets

| Target | Authorization basis | Testing window | Status |
| --- | --- | --- | --- |
| Pending | Pending written owner authorization | Pending | Not authorized |

## Excluded Assets

- Public IP ranges not owned by the repository owner.
- Third-party domains, APIs, infrastructure, and accounts.
- Production systems unless explicitly added to this file with written authorization.
- Personal accounts, customer accounts, and real user credentials.
- Cloud accounts or SaaS tenants not explicitly listed as authorized.
- Other LAN hosts.
- Other ports on `192.168.12.192` unless explicitly added later.
- Public IPs.
- Third-party domains.

## Allowed Tools

- Go native tests and vetting.
- govulncheck when available.
- gitleaks secret scanning.
- semgrep SAST.
- trivy filesystem, dependency, and image scanning.
- syft SBOM generation.
- grype or trivy vulnerability matching.
- OWASP ZAP baseline scan against local or explicitly scoped targets.
- nuclei templates only against local or explicitly scoped targets.
- Pentest-Swarm-AI-style validation only against targets explicitly listed as authorized in this file.

## Allowed Live Testing

- HTTPS checks against `https://192.168.12.192:8989`.
- HTTP checks against `http://192.168.12.192:8989` for protocol validation only.
- Nmap service/version check on port `8989` for `192.168.12.192`.
- Nuclei low-rate templates against `https://192.168.12.192:8989`.
- OWASP ZAP baseline scan against `https://192.168.12.192:8989`.

## Forbidden Actions

- Unauthorized reconnaissance.
- Public target scanning.
- Brute force against real accounts.
- Destructive exploitation.
- Persistence.
- Credential theft.
- Secret dumping.
- Data exfiltration.
- Malware.
- Backdoors.
- Disabling or bypassing logging.
- Stealth activity.

## Rate Limits

- Local API testing: keep requests low volume and stop on errors or instability.
- Authorized live target `https://192.168.12.192:8989`: keep checks low-rate and non-destructive.
- Staging testing: Pending owner-defined rate limits before authorization.
- Nuclei and ZAP: use baseline, non-destructive templates only unless a separate written approval is added.

## Evidence Handling

- Store command evidence under `security/evidence/commands.log`.
- Store scan outputs under `security/evidence/scans/`.
- Store SBOM outputs under `security/evidence/sbom/`.
- Store reports under `security/reports/`.
- Mark missing or unvalidated evidence as `Pending`.
