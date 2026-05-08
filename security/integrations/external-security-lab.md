# External Security Lab Integration

## Status

Pending external lab initialization.

## Role of `apig0-security-lab`

`apig0-security-lab` is the expected external security validation environment for Apig0. It may host orchestration and tooling such as Vigil, OWASP ZAP, Nmap, ffuf, Trivy, Gitleaks, Nikto, OpenSSL, and other third-party security utilities.

Apig0 remains the primary product repository. The external lab should run advanced validation and export evidence back to this repository only when artifacts are relevant to product security readiness.

## Separation of Responsibilities

| Repository | Responsibility |
| --- | --- |
| Apig0 | Product code, policies, findings, remediation tracking, compliance mappings, lightweight local scripts, and imported evidence |
| `apig0-security-lab` | Advanced scan orchestration, third-party tooling configuration, raw scan execution, and external validation workflows |

Do not clone, vendor, or commit large third-party security tooling into the Apig0 product repository.

## Evidence Export Flow

1. Run external security validation from `apig0-security-lab`.
2. Export reports and supporting artifacts to the lab output directory.
3. Review artifacts for relevance and sensitive data.
4. Import approved artifact types with `security/integrations/import-evidence.sh`.
5. Update `security/reports/findings.md` only after validation.
6. Link evidence IDs to findings, remediation entries, and control mappings.

## Scan References

Imported evidence must disclose:

- Source repository or environment
- Tool name and version when available
- Target URL or host
- Date and time of execution
- Operator or automation identity
- Validation status
- Whether findings are pending validation

Suggested wording:

> Security validation activities were performed using external tooling from the apig0-security-lab environment, including Vigil and OWASP-aligned scanning utilities.

## Third-Party Tool Attribution

Vigil, OWASP ZAP, Nmap, ffuf, Trivy, Gitleaks, Nikto, OpenSSL, and similar tools are third-party or external validation utilities. Do not describe them as proprietary Apig0 components.

## Traceability

All imported evidence should be indexed in `security/evidence/evidence-index.md`. Findings should reference the imported artifact path and remain `Pending Validation` until reviewed.
