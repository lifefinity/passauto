# Architecture

## Overview

passauto is a CLI tool that wraps interactive commands in a pseudo-terminal, watches their output for configurable patterns, and automatically responds with encrypted secrets. It functions like Linux `expect` but with built-in secret management.

## High-Level Flow

```
┌─────────────────────────────────────────────────────────┐
│                      User                                │
│   passauto myserver --then "ls" --then "exit"            │
└─────────────────────┬───────────────────────────────────┘
                      │
                      ▼
┌─────────────────────────────────────────────────────────┐
│                   cmd/ (CLI Layer)                        │
│                                                          │
│  root.go: Dispatch profile name → runProfileWithSteps()  │
│  run.go:  Decrypt secret, build engine.Options           │
│  version.go: Print version/commit/build info             │
│  helpers.go: Load data, derive encryption key            │
│              ├─ Check keychain cache (OS keyring)         │
│              ├─ SSH mode: HKDF derive → cache key         │
│              └─ KMS mode: KMS Decrypt(encrypted DEK)      │
└─────────────────────┬───────────────────────────────────┘
                      │
                      ▼
┌─────────────────────────────────────────────────────────┐
│              internal/engine/ (Execution Engine)          │
│                                                          │
│  engine.go:       Run() entry point                      │
│  pty_windows.go:  ConPTY process + I/O loop              │
│  pty_unix.go:     PTY process + I/O loop                 │
│  matcher.go:      Regex pattern matching                 │
│  stepper.go:      Post-login command sequencing          │
└─────────────────────┬───────────────────────────────────┘
                      │
                      ▼
┌─────────────────────────────────────────────────────────┐
│              Pseudo-Terminal (ConPTY / PTY)               │
│                                                          │
│  Child process (kinit, ssh, psql, etc.)                 │
└─────────────────────────────────────────────────────────┘
```

## Component Details

### CLI Layer (`cmd/`)

Responsible for:
- Parsing commands and flags (via Cobra)
- Loading profile data from disk
- Deriving encryption key from SSH private key
- Decrypting stored secrets
- Building `engine.Options` and calling `engine.Run()`

Key dispatch logic:
- `passauto <name>` → `root.go` → `runProfileWithSteps()` (loads profile, decrypts, runs)
- `passauto add/update/list/remove/version` → respective command handlers

### Engine (`internal/engine/`)

The core execution engine. Platform-agnostic interface with platform-specific PTY implementations.

#### engine.go

- `Run(opts Options)` — entry point
- `stripAnsi(s string)` — removes ANSI escape sequences before matching
- Delegates to `runWithPTY()` (platform-specific)

#### matcher.go

- Compiles regex patterns at startup
- `Check(line string) *MatchResult` — tests a line against all patterns
- Returns the response string and hidden flag on match

#### stepper.go

- Manages post-login command sequences (`--then`, `--script`)
- Activated after the first pattern match (password sent)
- Watches for shell prompt pattern, sends next command in sequence
- Thread-safe (mutex-protected)

#### pty_windows.go / pty_unix.go

Platform-specific PTY management:

**Windows (ConPTY):**
```
CreatePipe (input) → pipeIn (we write) / ptyIn (PTY reads)
CreatePipe (output) → pipeOut (we read) / ptyOut (PTY writes)
CreatePseudoConsole(size, ptyIn, ptyOut)
CreateProcess with PROC_THREAD_ATTRIBUTE_PSEUDOCONSOLE
```

**I/O Loop (both platforms):**
```
Goroutine 1: stdin → pipeWriter (forward user input)
Goroutine 2: pipeReader → stdout + pattern matching
  - Accumulates output in line buffer
  - On newline: process complete line
  - On 100ms timeout: process partial line (handles prompts without newline)
  - Strips ANSI codes before matching
  - On match: writes response to pipeWriter
```

### Crypto (`internal/crypto/`)

#### Encryption Modes

passauto supports two encryption paths:

1. **SSH-derived (default)** — Key derived from local SSH private key via HKDF. No network calls, works offline.
2. **KMS envelope (team/enterprise)** — AWS KMS generates a data encryption key (DEK). Encrypted DEK stored alongside ciphertext.

```
┌─────────────── Mode 1: SSH-Derived ────────────────┐
│                                                     │
│  SSH Private Key File                               │
│         │                                           │
│         ├─ Parse with ssh.ParseRawPrivateKey()      │
│         │                                           │
│         ▼                                           │
│     Raw key bytes                                   │
│         │                                           │
│         ├─ HKDF-SHA256                              │
│         │   Salt: "passauto-salt-v1"                │
│         │   Info: "passauto-v1"                     │
│         ▼                                           │
│     256-bit AES key                                 │
│         │                                           │
│         ├──► Keychain Cache (OS keyring, 1h TTL)    │
│         │    (skip HKDF on subsequent runs)         │
│         │                                           │
│         ├─ Encrypt: AES-256-GCM (random nonce)     │
│         └─ Decrypt: AES-256-GCM                    │
│                                                     │
└─────────────────────────────────────────────────────┘

┌─────────────── Mode 2: KMS Envelope ───────────────┐
│                                                     │
│  AWS KMS Key ARN                                    │
│         │                                           │
│         ├─ GenerateDataKey (AES-256)                │
│         │   Returns: plaintext DEK + encrypted DEK  │
│         ▼                                           │
│     Plaintext DEK (in-memory only)                  │
│         │                                           │
│         ├─ Encrypt: AES-256-GCM (random nonce)     │
│         │   Store: encrypted DEK + nonce + cipher   │
│         │                                           │
│         ├─ Decrypt: KMS Decrypt(encrypted DEK)      │
│         │   → plaintext DEK → AES-256-GCM decrypt  │
│         │                                           │
└─────────────────────────────────────────────────────┘
```

### Data (`internal/data/`)

Single JSON file at `~/.passauto/data.json` (permissions 0600):

```json
{
  "ssh_key": "~/.ssh/id_ed25519",
  "profiles": {
    "prod": {
      "command": "ssh deploy@prod-server",
      "patterns": [{"match": "(?i)password:", "hidden": true}],
      "secret": "base64(nonce+ciphertext+tag)",
      "timeout": "30s"
    },
    "mydb": {
      "command": "psql -h db.example.com -U admin mydb",
      "patterns": [{"match": "(?i)password", "hidden": true}],
      "secret": "base64(nonce+ciphertext+tag)",
      "prompt": "=>\\s*$",
      "timeout": "30s"
    }
  }
}
```

Responsibilities:
- Load/Save with JSON marshaling
- Validate regex patterns on load
- Prevent reserved names (add, update, list, remove, version, init, help)
- CRUD operations on profiles

## Data Flow: Pattern Matching

```
PTY Output (raw bytes)
       │
       ▼
   Accumulate in line buffer
       │
       ├─ Newline found? → Extract complete line
       │
       └─ 100ms timeout? → Flush partial line
              │
              ▼
       Strip ANSI escape codes
              │
              ▼
       Matcher.Check(cleanLine)
              │
              ├─ Match found → Write response + "\r\n" to PTY input
              │                 Activate Stepper
              │
              └─ No match → Stepper.Check(cleanLine)
                                    │
                                    ├─ Prompt matched + steps remaining
                                    │   → Write next step + "\r\n"
                                    │
                                    └─ No match → do nothing
```

## Security Model

1. **No plaintext secrets on disk** — all secrets encrypted with AES-256-GCM
2. **Two encryption modes** — SSH key derivation (offline, single-user) or KMS envelope encryption (team/enterprise, IAM-controlled)
3. **Key never stored in plaintext** — SSH-derived key cached in OS keychain (1h TTL) or held in-memory only (KMS mode)
4. **Per-secret nonce** — each encryption uses a unique random nonce
5. **Keychain cache** — derived AES key cached per-profile in OS keyring (macOS Keychain, Linux secret-service, Windows Credential Manager); bypass with `--no-cache`
6. **File permissions** — data.json created with 0600 (owner read/write only)
7. **Hidden input** — secrets read via `term.ReadPassword()` (no echo)
8. **Hidden response** — when `hidden: true`, the response is not echoed to terminal output

## Concurrency Model

The engine uses goroutines for non-blocking I/O:

```
Main goroutine:
  └─ WaitForSingleObject (process exit)
  └─ ClosePseudoConsole (triggers reader EOF)
  └─ wg.Wait()

Goroutine 1 (input forwarder):
  └─ io.Copy(pipeWriter, stdin)

Goroutine 2 (output reader + matcher):
  └─ Async read loop with select
  └─ Timer-based flush for partial lines
  └─ Pattern matching + response writing
```

The Stepper is mutex-protected for thread safety since it can be accessed from the output reader goroutine.
