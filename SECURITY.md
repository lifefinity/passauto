# Security Policy

## Supported Versions

| Version | Supported          |
|---------|--------------------|
| latest  | :white_check_mark: |

## Reporting a Vulnerability

If you discover a security vulnerability in passauto, please report it responsibly:

1. **Do NOT** open a public GitHub issue
2. Use [GitHub Security Advisories](https://github.com/lifefinity/passauto/security/advisories/new) to report privately
3. Include:
   - Description of the vulnerability
   - Steps to reproduce
   - Potential impact
   - Suggested fix (if any)

## Response Timeline

- **Acknowledgment**: Within 48 hours
- **Initial assessment**: Within 7 days
- **Fix release**: Within 30 days for critical issues

## Scope

Security issues we care about:
- Secret leakage (decrypted secrets exposed in memory/logs/disk)
- Encryption weaknesses (key derivation, AES-GCM usage)
- PTY escape / command injection
- Dependency vulnerabilities

Out of scope:
- Attacks requiring physical access to an unlocked machine
- Social engineering
