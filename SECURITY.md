# Security Policy

## Supported Versions

| Version | Supported          |
|---------|:------------------:|
| 0.6.x   | ✅ Current          |
| < 0.6   | ❌ No longer supported |

## Reporting a Vulnerability

**Please do NOT report security vulnerabilities through public GitHub issues.**

Instead, report them privately via one of these methods:

1. **GitHub Security Advisories**: Go to [Security → Advisories → New draft advisory](https://github.com/jmrplens/portainer-mcp-enhanced/security/advisories/new)
2. **Email**: Send details to the repository owner listed in [CODEOWNERS](.github/CODEOWNERS)

### What to include

- Description of the vulnerability
- Steps to reproduce
- Affected versions
- Potential impact
- Suggested fix (if any)

### Response timeline

- **Acknowledgment**: Within 48 hours
- **Initial assessment**: Within 1 week
- **Fix or mitigation**: Depends on severity (critical: ASAP, high: 2 weeks, medium/low: next release)

### Disclosure policy

- We follow [coordinated disclosure](https://en.wikipedia.org/wiki/Coordinated_vulnerability_disclosure)
- A fix will be prepared before any public disclosure
- Credit will be given to the reporter (unless they prefer anonymity)

## Security Best Practices for Users

- **API tokens**: Use least-privilege tokens; rotate them regularly
- **TLS**: Always use HTTPS connections to your Portainer instance; avoid `-skip-tls-verify` in production
- **Read-only mode**: Use `-read-only` for monitoring-only deployments
- **Network**: Run the MCP server in a trusted network segment; do not expose it to the public internet
- **Docker**: If running via Docker, avoid `--privileged` and limit mounted volumes
