# Expected External Security Lab Artifacts

## Status

Pending external lab initialization.

`apig0-security-lab` may export the following artifact types for import into Apig0. Artifacts should be reviewed for secrets and relevance before import.

| Artifact Type | Examples | Expected Location After Import | Status |
| --- | --- | --- | --- |
| SARIF reports | `gosec.sarif`, `gitleaks.sarif`, custom static-analysis SARIF | `security/evidence/imported/` | Pending |
| OWASP ZAP reports | HTML, JSON, XML, Markdown reports | `security/evidence/imported/` | Pending |
| Nmap outputs | Normal, XML, grepable, or text output | `security/evidence/imported/` | Pending |
| Trivy reports | JSON, SARIF, table output | `security/evidence/imported/` | Pending |
| CVSS findings | Markdown, JSON, CSV exports | `security/evidence/imported/` and `security/reports/findings.md` after validation | Pending |
| Screenshots | PNG, JPG, JPEG, WebP | `security/evidence/imported/` or `security/evidence/screenshots/` | Pending |
| Remediation exports | Markdown, CSV, JSON | `security/evidence/imported/` and `security/reports/remediation-plan.md` after validation | Pending |
| Evidence bundles | ZIP, tarball, directory exports | `security/evidence/imported/` | Pending |

## Approved Import Extensions

The import script accepts common evidence formats only:

- `.sarif`
- `.json`
- `.xml`
- `.html`
- `.md`
- `.txt`
- `.log`
- `.csv`
- `.png`
- `.jpg`
- `.jpeg`
- `.webp`
- `.pdf`
- `.zip`
- `.tar`
- `.tar.gz`
- `.tgz`

Rejected files should remain in the external lab until reviewed.
