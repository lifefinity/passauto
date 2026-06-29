# passauto — Automated Interactive Prompt Responder

## Overview

A Go CLI tool that wraps commands in a pseudo-terminal (PTY), watches output for configured regex patterns, and automatically responds with decrypted secrets. Functions like Linux `expect` but with encrypted secret storage and declarative YAML configuration.

## Requirements

- Cross-platform: Linux, macOS, Windows (ConPTY)
- Single static binary, no runtime dependencies
- Secrets encrypted at rest using AES-256-GCM with key derived from SSH private key
- Configurable via YAML profiles or ad-hoc command wrapping
- General-purpose pattern matching (passwords, confirmations, any prompt)
- Transparent passthrough of unmatched output and user input

## Architecture

```
┌─────────────────────────────────────────────────┐
│                   passauto CLI                    │
├─────────────┬──────────────┬────────────────────┤
│  Config     │  Crypto      │  PTY Engine        │
│  (YAML)     │  (AES-GCM)  │  (creack/pty +     │
│             │              │   conpty)           │
├─────────────┴──────────────┴────────────────────┤
│              Secret Store (~/.passauto/)          │
└─────────────────────────────────────────────────┘
```

### Components

1. **CLI** — cobra-based command parsing with subcommands
2. **Config** — YAML profile/pattern loader and validator
3. **Crypto** — SSH key-based key derivation, AES-GCM encrypt/decrypt
4. **PTY Engine** — cross-platform PTY spawning, output buffering, pattern matching, response injection
5. **Secret Store** — encrypted JSON blob at `~/.passauto/secrets.enc`

## CLI Interface

```
passauto init                     # First-time setup: select SSH key, derive encryption key
passauto add <name>               # Store a new secret (prompts for value securely)
passauto list                     # List stored secret names (not values)
passauto remove <name>            # Delete a stored secret
passauto run <command...>         # Ad-hoc: wrap a command with pattern matching from defaults
passauto <profile>                # Run a named profile from config (profile names must not collide with subcommands)
```

### Usage Examples

```bash
# First-time setup
passauto init
# → Select SSH key: ~/.ssh/id_ed25519
# → Enter passphrase (if key is encrypted): ****
# → Initialized. Config at ~/.passauto/config.yaml

# Store a password
passauto add midway-password
# → Enter secret: ****
# → Stored "midway-password" (encrypted)

# Ad-hoc usage (patterns from config's "defaults" rules)
passauto run mwinit -s -o

# Profile usage
passauto mwinit
# → Expands to `mwinit -s -o` with profile-specific patterns
```

## Configuration

Location: `~/.passauto/config.yaml`

```yaml
ssh_key: ~/.ssh/id_ed25519

# Default patterns applied to all commands unless overridden
defaults:
  patterns:
    - match: "(?i)password:"
      respond: "{{midway-password}}"
      hidden: true          # don't echo response to terminal

# Named profiles
profiles:
  mwinit:
    command: "mwinit -s -o"
    patterns:
      - match: "(?i)password:"
        respond: "{{midway-password}}"
        hidden: true
      - match: "(?i)\\(yes/no\\)"
        respond: "yes"
    timeout: 30s            # max time to wait for each pattern

  kinit:
    command: "kinit"
    patterns:
      - match: "(?i)password for"
        respond: "{{kerberos-password}}"
        hidden: true
```

### Config Semantics

- Profile names must not collide with subcommands (`init`, `add`, `list`, `remove`, `run`); config validation rejects these
- `{{secret-name}}` references encrypted secrets from the store
- `hidden: true` suppresses echoing the response to the user's terminal
- `timeout` per profile, defaults to 30s
- Patterns are Go regex, matched against output line-by-line
- Patterns tested in config order; first match wins

## Crypto Design

### Key Derivation

1. Read SSH private key file (e.g., `~/.ssh/id_ed25519`)
2. If passphrase-protected, prompt user for passphrase to decrypt it
3. Extract raw private key bytes
4. Derive 256-bit AES key using HKDF-SHA256 with salt `"passauto-salt-v1"` and info `"passauto-v1"`

### Encryption

- Each secret encrypted individually with AES-256-GCM
- Random 12-byte nonce per encryption operation
- Stored format per secret: `nonce (12 bytes) || ciphertext || GCM tag (16 bytes)`

### Secret Store File

Location: `~/.passauto/secrets.enc`, permissions `0600`

```json
{
  "midway-password": "base64(nonce+ciphertext+tag)",
  "kerberos-password": "base64(nonce+ciphertext+tag)"
}
```

### Security Properties

- Secrets encrypted at rest; compromising the file alone is insufficient
- Encryption key never stored on disk; derived on-the-fly from SSH key
- If SSH key has a passphrase, user types it once per invocation; derived key cached in memory for process lifetime only
- Decrypted secrets zeroed from memory after child exits (best-effort via manual zeroing)
- File permissions enforced on creation; warn if too open

## PTY Engine

### Process Flow

```
1. Parse command + resolve profile
2. Derive encryption key from SSH key
3. Decrypt needed secrets into memory
4. Spawn child process in PTY
5. Loop:
   a. Read output from PTY (non-blocking, buffered)
   b. Forward output to user's terminal (passthrough)
   c. Check accumulated line buffer against patterns
   d. On match: write response to PTY stdin, clear matched buffer
   e. If no patterns pending + child exited → done
6. Exit with child's exit code
```

### Platform-Specific PTY

- **Linux/macOS:** `github.com/creack/pty` library
- **Windows:** `conpty` package using Windows Pseudo Console API (Windows 10 1809+)

### Key Behaviors

- **Passthrough:** All child output shown to user in real time
- **User input forwarding:** Unmatched prompts pass user keystrokes through to child
- **Terminal resize:** SIGWINCH (Unix) / console events (Windows) forwarded to child PTY
- **Signal forwarding:** SIGINT, SIGTERM forwarded to child process
- **Timeout:** If pattern not matched within timeout, control passes to user (no hang)
- **Exit code:** passauto exits with child's exit code
- **Partial line matching:** Short delay (100ms) after output without newline to detect prompts that don't end with newline

### Pattern Matching Strategy

- Buffer output line-by-line
- After each new line (or partial line after delay), test all active patterns in config order
- First match wins
- After responding, wait configurable delay (default 100ms) before resuming matching

## Error Handling

### Startup Errors

- SSH key not found → clear error, suggest `passauto init`
- SSH key passphrase wrong → retry up to 3 times, then fail
- Config malformed → error with context
- Referenced secret not in store → error naming the missing secret before spawning child

### Runtime Errors

- Child process crashes → forward exit code, clean up PTY
- Pattern timeout → print warning, fall through to user input (don't kill process)
- PTY allocation fails → error message, no silent fallback

### Security Edge Cases

- Secrets never in logs, env vars, or temp files
- Warn if `~/.passauto/` permissions are world-readable
- Config can reference nonexistent secrets → error at run time, not parse time

### Graceful Degradation

- Non-interactive terminal (piped) → warn and proceed
- ConPTY unavailable on Windows → error with minimum version requirement

## Testing Strategy

### Unit Tests

- **Crypto:** Round-trip encrypt/decrypt, key derivation determinism, invalid key handling
- **Config parser:** Valid YAML, missing fields, invalid regex, template resolution
- **Pattern matcher:** Regex matching, match ordering, partial lines, timeout

### Integration Tests

- **PTY engine:** Spawn test program that prints "Password:" and reads stdin, verify response
- **End-to-end profiles:** Mock interactive program, verify multi-prompt sequencing
- **Cross-platform CI:** Matrix for Linux, macOS, Windows

### Manual Test Scenarios

- `mwinit -s -o` with real Midway password
- Multi-prompt program (password + confirmation)
- Timeout behavior for unconfigured prompts
- User input passthrough for unmatched prompts

## Dependencies (Go)

- `github.com/spf13/cobra` — CLI framework
- `github.com/creack/pty` — Unix PTY
- `github.com/UserExistsError/conpty` (or similar) — Windows PTY
- `golang.org/x/crypto/ssh` — SSH key parsing
- `golang.org/x/crypto/hkdf` — key derivation
- `crypto/aes`, `crypto/cipher` — AES-GCM (stdlib)
- `gopkg.in/yaml.v3` — YAML parsing
- `golang.org/x/term` — terminal raw mode, secure input

## File Layout

```
~/.passauto/
├── config.yaml       # Profiles and patterns
└── secrets.enc       # Encrypted secrets (JSON, 0600)
```

## Project Structure

```
passauto/
├── cmd/
│   ├── root.go       # Cobra root command
│   ├── init.go       # passauto init
│   ├── add.go        # passauto add
│   ├── list.go       # passauto list
│   ├── remove.go     # passauto remove
│   └── run.go        # passauto run + profile dispatch
├── internal/
│   ├── config/       # YAML config loading and validation
│   ├── crypto/       # Key derivation, encrypt, decrypt
│   ├── engine/       # PTY engine, pattern matcher
│   └── store/        # Secret store read/write
├── main.go
├── go.mod
└── go.sum
```
