# Security Policy

## Supported Versions

| Version | Supported |
|---------|-----------|
| latest  | ✅ Yes    |
| < 1.0   | ❌ No     |

## Scope

PathCollapse is a **read-only analytics tool**. It ingests graph data and produces reports; it performs no network scanning, credential access, or system modification. Security issues in scope include:

- Path injection in file loading (`--graph`, `--baseline`, `--output`)
- Malicious graph data causing out-of-memory or crash
- Information disclosure in generated reports
- Dependency vulnerabilities in the Go module graph

## Reporting a Vulnerability

Please **do not** open a public GitHub issue for security vulnerabilities.

Email: **kartikarunapuram@gmail.com**

Include:
- Affected version / commit
- Description of the vulnerability
- Steps to reproduce
- Potential impact assessment
- Any suggested mitigations

You can expect an acknowledgement within **72 hours** and a status update within **14 days**.

## Disclosure Policy

- We follow [coordinated disclosure](https://en.wikipedia.org/wiki/Coordinated_vulnerability_disclosure).
- We will credit reporters in the release notes unless anonymity is requested.
- Patches will be released as soon as a fix is validated.

## Dependency Scanning

Dependencies are scanned via `go mod verify` in CI and periodic `govulncheck` runs.
