# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Shell completion (bash, zsh, fish, powershell)
- Export/import profiles (JSON, `--force` overwrite)
- Backup/restore encrypted data file
- `--dry-run` flag to preview without executing
- `--case-sensitive` flag for pattern matching
- `--quiet` / `-q` mode to suppress output
- `change-key` command for key rotation
- Auto-init on first use (generates ed25519 key)
- Passphrase protection for encryption key (`--no-passphrase` to skip)
- `--key` flag for custom key path
- PR comment commands: `/rerun`, `/test`, `/benchmark`
- CodeQL weekly scanning
- Codecov coverage reporting
- PR auto-labeler
- golangci-lint v2 configuration

### Fixed
- Go 1.25.3 → 1.25.8 (GO-2026-4602 os.ReadDir vulnerability)
- gosec false positive suppression with `#nosec` annotations
- PTY read loop refactored for stability
- Memory zeroing for decrypted secrets

### Security
- Encrypted secret storage (ed25519 + AES-GCM)
- gosec + govulncheck in CI pipeline
- Dependabot for gomod and GitHub Actions

## [0.1.0] - 2025-05-25

### Added
- Initial release
- Profile-based prompt automation with encrypted secrets
- Multi-pattern matching per profile
- Post-login steps and after-commands
- Cross-platform support (Linux, macOS, Windows)
