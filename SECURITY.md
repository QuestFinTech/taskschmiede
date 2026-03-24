# Security Policy

## Reporting a Vulnerability

If you discover a security vulnerability in Taskschmiede, please report it responsibly.

**Do NOT open a public GitHub issue for security vulnerabilities.**

Instead, please use our contact form at: **https://taskschmiede.com/contact**

Include:
- Description of the vulnerability
- Steps to reproduce
- Potential impact
- Suggested fix (if any)

## Response Timeline

- **Acknowledgment**: Within 48 hours of receipt
- **Initial assessment**: Within 5 business days
- **Fix timeline**: Depends on severity; critical issues are prioritized

## Scope

The following are in scope:
- Authentication and authorization bypass
- Cross-site scripting (XSS) and cross-site request forgery (CSRF)
- SQL injection
- MCP protocol security (JSON-RPC validation, tool authorization)
- Session management vulnerabilities
- Privilege escalation (cross-org access, role bypass)
- Content injection and prompt injection in onboarding flows
- Denial of service via resource exhaustion

The following are out of scope:
- Vulnerabilities in dependencies (report to the upstream project)
- Social engineering attacks
- Denial of service via network flooding (infrastructure concern)
- Issues requiring physical access to the server

## Supported Versions

Security fixes are applied to the latest release only. We recommend always running the most recent version.

## Security Measures

Taskschmiede includes several built-in security measures:

- **Rate limiting**: Per-IP, per-session, and per-auth-endpoint rate limits
- **Connection limits**: Global and per-IP connection caps
- **Body size limits**: Request body size enforcement on all endpoints
- **Content Security Policy**: Configurable CSP, HSTS, and security headers
- **Input validation**: Server-side validation on all inputs
- **Parameterized queries**: All database queries use parameterized statements
- **Password policy**: Minimum 12 characters with complexity requirements
- **Session management**: Database-backed sessions with automatic expiry
- **Audit logging**: All authentication and entity changes are logged
- **MCP security**: Optional protocol-level validation and per-tool rate limits

## Automated Scanning

The CI pipeline includes:
- `govulncheck` for known Go vulnerability detection
- `gosec` for static security analysis
- Dependency audit via `go mod verify`
