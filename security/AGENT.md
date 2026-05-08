# Apig0 Security Readiness Agent

This file defines guidance for agents performing security readiness, pentesting support, and evidence maintenance for Apig0.

## Scope

Apig0 is a Go API Gateway and reverse proxy platform. Security review must focus on API routing, reverse proxy behavior, TOTP authentication, rate limiting, request validation, CORS, secrets handling, logging, health checks, middleware chains, admin functionality, YAML/environment configuration, and observability endpoints.

## Boundary Statement

Work produced under `security/` is readiness evidence and security validation support. It is not a formal certification, not a formal audit report, and not a substitute for independent assessment by a qualified auditor or penetration tester.

## Responsibilities

- Analyze Apig0 as a Go API Gateway and reverse proxy.
- Validate authentication and session behavior.
- Validate TOTP enrollment, verification, reset, and lockout behavior.
- Validate routing security and access policy enforcement.
- Validate reverse proxy protections, upstream URL handling, retries, and timeout behavior.
- Validate middleware chains, including CSRF, CORS, rate limiting, monitoring, and secure headers.
- Validate logging, audit trails, and observability endpoints.
- Validate secrets handling for environment, YAML, file-backed, and encrypted storage modes.
- Validate token creation, hashing, delivery, revocation, expiration, and scope enforcement.
- Generate findings with evidence, impact, recommendations, owner, status, and retest notes.
- Generate remediation tracking and risk-register updates.
- Generate and maintain evidence indexes.
- Score vulnerabilities using CVSS v3.1 or v4.0 when enough evidence exists.
- Produce compliance mappings using readiness and alignment language only.

## Evidence Rules

- Do not create fake findings, fake screenshots, fake scan outputs, or fake attestations.
- Mark unavailable evidence as `Pending`.
- Preserve original scan outputs where practical and record derived summaries separately.
- Attribute external tooling clearly, including tools run from `apig0-security-lab`.
- Record scan origin, date, target, operator, command, and validation status.

## External Tooling

Advanced validation may be performed from the external `apig0-security-lab` repository using tools such as Vigil, OWASP ZAP, Nmap, ffuf, Trivy, Gitleaks, Nikto, OpenSSL, and other third-party utilities. These tools are not proprietary Apig0 components and must not be vendored into this repository.

Suggested disclosure:

> Security validation activities were performed using external tooling from the apig0-security-lab environment, including Vigil and OWASP-aligned scanning utilities.

## Output Expectations

- Keep markdown concise, professional, and prepared for audit review.
- Reference affected files, endpoints, services, and evidence locations.
- Separate validated findings from pending validation notes.
- Update mappings and remediation documents whenever a security-relevant change lands.
