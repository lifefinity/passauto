# User Guide

## Installation

```bash
cd projects/passauto
go build -o passauto.exe .
```

Or use the Makefile:

```bash
make build        # Output: bin/passauto.exe
make install      # Install to GOPATH/bin
```

## First-Time Setup

On first use, passauto will auto-initialize. You can also run manually:

```bash
passauto init
```

This creates `~/.passauto/data.json` and configures encryption.

### How `init` works

1. If `--key <path>` is specified, uses that SSH key
2. Otherwise looks for `~/.ssh/id_ed25519`, `id_rsa`, or `id_ecdsa`
3. If no SSH key is found, generates a dedicated key at `~/.passauto/passauto_key`

If the selected key is passphrase-protected, you will be prompted once to verify access.

### Init Flags

| Flag | Description |
|------|-------------|
| `--key <path>` | Use an existing SSH private key instead of auto-detecting |
| `--no-passphrase` | Skip passphrase prompt for generated key |

### Examples

```bash
passauto init                              # Auto-detect or generate
passauto init --key ~/.ssh/id_ed25519      # Use a specific key
passauto init --no-passphrase              # Generate key without passphrase
```

## Adding a Profile

### Interactive Mode

```bash
passauto add myserver
```

Prompts for command, pattern, and secret.

### With Flags

```bash
passauto add -c "ssh deploy@prod-server" -m "password:" -d "Production server" prod
passauto add -c "psql -h db.example.com -U admin mydb" -m "password" -p "=>\s*$" -d "Main database" mydb
passauto add -c "kinit admin@CORP.COM" -m "Password:" -d "Kerberos auth" --after "klist" krb
```

### Add Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--command` | `-c` | Command to execute |
| `--desc` | `-d` | Short description (shown in `passauto list`) |
| `--match` | `-m` | Regex pattern to match (can specify multiple) |
| `--prompt` | `-p` | Shell prompt regex for post-login `--then` steps |
| `--timeout` | `-t` | Timeout per match (default: `30s`) |
| `--case-sensitive` | | Match with exact case (default: case-insensitive) |
| `--then` | | Command to send inside session after login (can specify multiple) |
| `--after` | | Command to run in new shell after profile exits (can specify multiple) |
| `--service` | `-s` | Service type for multi-service disambiguation (e.g. ssh, pg, oracle) |
| `--kms-key-id` | | AWS KMS key ARN for envelope encryption (replaces SSH key derivation) |
| `--no-cache` | | Bypass keychain cache, re-derive key from SSH key |

### --then vs --after

| | `--then` | `--after` |
|---|----------|-----------|
| **When** | Inside the running session | After process exits (exit code 0) |
| **Requires** | `-p` prompt pattern | Nothing |
| **Use for** | psql, mysql, ssh shell | kinit, docker login, one-shot auth |
| **Runs in** | The PTY session | A new `sh -c` shell |

### Example: PostgreSQL with built-in steps

```bash
passauto add -c "psql -h localhost -U admin mydb" \
  -m "password" -p "=>\s*$" \
  --then "\timing on" --then "SET search_path TO public;" \
  -d "Local PG with timing" mydb
```

### Example: kinit with post-exit command

```bash
passauto add -c "kinit admin@CORP.COM" -m "Password:" \
  --after "date" --after "echo 'Midway refreshed'" \
  -d "Kerberos auth" krb
```

## Multi-Service Profiles

When a server has multiple services (SSH, PostgreSQL, Oracle, etc.), use `-s` to store them under the same name:

```bash
passauto add -c "ssh admin@prod" -m "password:" -s ssh prod
passauto add -c "psql -h prod -U admin" -m "password" -s pg prod

passauto prod -s ssh   # run SSH profile
passauto prod -s pg    # run PostgreSQL profile
passauto prod          # if ambiguous, shows selection menu
```

Uniqueness is enforced on `(name, service)` pairs. Profiles without `-s` have an empty service field.

## KMS Envelope Encryption

For team/enterprise use, passauto supports AWS KMS instead of SSH key derivation:

```bash
# New profile with KMS
passauto add -c "ssh admin@prod" -m "password:" --kms-key-id arn:aws:kms:us-east-1:123456:key/abc prod

# Switch existing profile to KMS
passauto update prod --kms-key-id arn:aws:kms:us-east-1:123456:key/abc
```

Requires AWS credentials and IAM permissions: `kms:GenerateDataKey`, `kms:Decrypt`.

## TOTP / 2FA Support

passauto can auto-fill time-based one-time passwords (TOTP/2FA). The TOTP secret seed is encrypted at rest alongside your password.

### TOTP-Only Profile

For services that only prompt for a verification code:

```bash
passauto add -c "vpn-connect" -m "Verification code:" myVPN --totp
# Enter secret: (press Enter — no password needed)
# Enter TOTP secret seed: <paste your base32 seed>
```

### Password + TOTP (Two-Step Auth)

For services that prompt for password first, then a TOTP code:

```bash
passauto add -c "ssh user@server" -m "password:" --totp-match "Verification code:" myserver
# Enter secret: <your password>
# Enter TOTP secret seed: <your base32 TOTP seed>
```

### Update TOTP

```bash
# Change the TOTP seed
passauto update myserver --totp-secret

# Change which prompt triggers TOTP
passauto update myserver --totp-match "OTP:"
```

### How It Works

- The TOTP seed (base32) is stored encrypted with the same key as your password
- At runtime, when a TOTP pattern matches, passauto generates a fresh 6-digit code (RFC 6238, HMAC-SHA1, 30-second period)
- The code is generated just-in-time — not before the prompt appears

### Finding Your TOTP Seed

The TOTP seed is the base32 string you normally scan as a QR code. Most authenticator apps let you view the secret key:
- Look for "Manual entry" or "Can't scan QR?" option during 2FA setup
- It's typically a string like `JBSWY3DPEHPK3PXP`

## Running a Profile

```bash
passauto <profile>
passauto <profile> --then "cmd"       # Additional session command
passauto <profile> --script file      # Commands from file
passauto <profile> --after "cmd"      # Run after process exits
passauto <profile> -e KEY=VALUE       # Inject environment variable
passauto <profile> --quiet            # Suppress terminal output
```

### Environment Variables

The child process inherits the current shell environment. Use `--env`/`-e` to inject additional variables:

```bash
passauto deploy -e HOST=prod.example.com -e PORT=5432
```

If the profile command uses shell variables (e.g., `ssh $USER@$HOST`), they are expanded at runtime from the inherited + injected environment.

## Post-Login Commands

After the password prompt is answered, chain commands inside the session:

```bash
passauto mydb --then "SELECT now();" --then "\q"
passauto mydb --script queries.sql
passauto mydb --then "\timing on" --script queries.sql --then "\q"
```

Profile-stored `--then` steps run first, then runtime `--then`/`--script` commands.

### Script File Format

One command per line. Empty lines and `#` comments are ignored:

```sql
# Enable timing
\timing on
SELECT * FROM users LIMIT 10;
\q
```

## Managing Profiles

```bash
passauto list                                    # List all profiles
passauto remove prod                             # Delete a profile
passauto update prod --secret                    # Change secret
passauto update prod -c "ssh newuser@host"       # Change command
passauto update prod -d "New description"        # Change description
passauto update prod --then "ls" --then "pwd"    # Set post-login steps
passauto update prod --after "notify-send done"  # Set post-exit commands
```

### Update Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--command` | `-c` | Update the command |
| `--desc` | `-d` | Update the description |
| `--match` | `-m` | Update the prompt pattern |
| `--prompt` | `-p` | Update the shell prompt pattern |
| `--timeout` | `-t` | Update the timeout |
| `--secret` | | Prompt for a new secret |
| `--case-sensitive` | | Enable exact-case matching |
| `--then` | | Replace post-login steps |
| `--after` | | Replace post-exit commands |

Only specified flags are changed; unspecified fields remain unchanged.

## Changing the Encryption Key

Switch all secrets to a new SSH key:

```bash
passauto change-key ~/.ssh/id_ed25519_new
```

This:
1. Decrypts all secrets with the current key (prompts for passphrase if needed)
2. Re-encrypts all secrets with the new key (prompts for passphrase if needed)
3. Updates the key reference in `data.json`

Both old and new keys can be passphrase-protected.

## Backup & Restore

### Backup

```bash
passauto backup /mnt/usb/passauto-backup
```

### Restore

```bash
passauto restore /mnt/usb/passauto-backup
passauto restore ~/backup --force           # Overwrite existing
```

## Export & Import

```bash
passauto export profiles.json              # Secrets excluded
passauto import profiles.json              # Merge with existing
passauto import profiles.json --force      # Overwrite on conflict
```

> Imported profiles have no secrets. Use `passauto update <name> --secret` to set them.

## Shell Completion

```bash
passauto completion bash    # eval "$(passauto completion bash)"
passauto completion zsh     # eval "$(passauto completion zsh)"
passauto completion fish    # passauto completion fish > ~/.config/fish/completions/passauto.fish
```

Tab-completion works for profile names: `passauto <TAB>`, `passauto update <TAB>`, `passauto remove <TAB>`.

## Pattern Tips

Patterns are **case-insensitive by default**. A partial match is enough.

| Pattern | Matches |
|---------|---------|
| `password` | "Password for user demo1:", "Enter password:", "PASSWORD:" |
| `pin` | "Midway PIN:", "Enter PIN:" |
| `Password for user .+:` | Any username in PostgreSQL prompts |
| `=>\s*$` | PostgreSQL shell prompt (for `-p` flag) |
| `\$\s*$` | Bash prompt |
| `mysql>` | MySQL prompt |

Use `--case-sensitive` only when you need exact matching.

## Troubleshooting

### "Profile not found"

Check with `passauto list`.

### Secret not being sent

1. Pattern may not match — try a broader regex (e.g. `password`)
2. ANSI escape codes are stripped before matching
3. Check timeout — increase with `passauto update <name> -t 60s`

### Process hangs after error

If the child process exits with an error (non-zero), passauto exits immediately. If it seems stuck, the child process may still be running — check with `ps`.

### SSH key passphrase

If your SSH key has a passphrase, passauto will prompt for it once per invocation.

## Platform Support

| Platform | Terminal Method |
|----------|---------------|
| Windows 10+ | ConPTY |
| Linux | PTY (creack/pty) |
| macOS | PTY (creack/pty) |
