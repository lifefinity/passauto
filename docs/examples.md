# Examples

Complete cookbook of passauto profiles for common tools.

## How It Works

```bash
# 1. Add a profile (one-time setup)
passauto add -c "ssh deploy@prod" -m "password:" myserver

# 2. Run it — password auto-filled every time
passauto myserver

# 3. Manage
passauto list              # show all profiles
passauto update myserver   # modify fields
passauto remove myserver   # delete
passauto myserver --dry-run  # preview without running
```

## Profile Naming

Each profile has a **unique name** — it is the key you use to run it. Names:
- Must be unique across all profiles (regardless of tool type)
- Must start with a letter/number, contain only `a-z A-Z 0-9 . - _`
- Cannot conflict with subcommands (`add`, `list`, `remove`, etc.)
- The same name can be reused with different `--service` (`-s`) values for multi-service disambiguation (e.g. `prod` with `-s ssh` and `prod` with `-s pg`)

If you try to add a name that already exists (with the same service), passauto will error and suggest `update` instead.

If you try to add a name that already exists, passauto will error and suggest `update` instead.

---

## SSH

```bash
# Add
passauto add -c "ssh deploy@prod-server" -m "password:" prod
passauto add -c "ssh -p 2222 admin@bastion.example.com" -m "password:" bastion
passauto add -c "ssh -J bastion db-internal" -m "password:" db-jump

# SSH key with passphrase (auto-fills the key unlock prompt)
passauto add -c "ssh -i ~/.ssh/deploy_key user@prod" -m "passphrase" prod-key

# Multi-service: same name, different service types
passauto add -c "ssh deploy@prod-server" -m "password:" -s ssh prod
passauto add -c "psql -h prod-db -U admin" -m "password" -s pg prod
passauto prod -s ssh    # connects via SSH
passauto prod -s pg     # connects to PostgreSQL

# Run
passauto prod           # connects to prod-server, auto-fills password
passauto prod-key       # unlocks private key, then connects
# Run
passauto prod           # connects to prod-server, auto-fills password
passauto bastion        # connects to bastion on port 2222
```

## Databases

```bash
# Add
passauto add -c "psql -h db.example.com -U admin mydb" -m "password" -p "=>\s*$" mydb
passauto add -c "mysql -h db.example.com -u root -p" -m "password:" mysql-prod
passauto add -c "sqlplus admin@orcl" -m "password:" oracle-prod
passauto add -c "mongosh mongodb://admin@db.example.com:27017/mydb" -m "password:" mongo-prod
passauto add -c "redis-cli -h cache.example.com" -m "password:" redis

# Multi-service: disambiguate same host by database type
passauto add -c "psql -h shared-db -U app" -m "password" -s pg shared-db
passauto add -c "mysql -h shared-db -u app -p" -m "password:" -s mysql shared-db
passauto shared-db -s pg      # PostgreSQL
passauto shared-db -s mysql   # MySQL

# Run
passauto mydb           # connects to PostgreSQL, auto-fills password
passauto mysql-prod     # connects to MySQL
passauto oracle-prod    # connects to Oracle
```

### --then vs --after

Two different hooks for two different moments:

| Flag | When | Use for |
|------|------|---------|
| `--then` | **Inside** the session, after password is filled | Run SQL, set config, execute commands in the shell |
| `--after` | **After** the process exits | Verify result, notify, chain next tool |

```
┌─────────────────────────────────────────────────────────┐
│  passauto mydb --then "\timing on" --after "echo done"  │
│                                                         │
│  1. Launch: psql -h db -U admin                         │
│  2. Auto-fill password when prompted                    │
│  3. --then: type "\timing on" into psql session  ← inside│
│  4. User interacts with psql...                         │
│  5. User exits psql (\q or Ctrl-D)                      │
│  6. --after: run "echo done" in parent shell     ← after│
└─────────────────────────────────────────────────────────┘
```

### Database with --then (Post-Login Commands)

```bash
# Run SQL after login
passauto mydb --then "SELECT now();" --then "\q"

# Enable timing + set schema (saved in profile)
passauto add -c "psql -h db.example.com -U admin mydb" -m "password" \
  -p "=>\s*$" --then "\timing on" --then "SET search_path TO app;" mydb

# Run a script file
passauto mydb --script queries.sql

# Combined
passauto mydb --then "\timing on" --script queries.sql --then "\q"
```

### Kerberos with --after (Post-Exit Commands)

```bash
# Verify ticket was obtained after kinit exits
passauto add -c "kinit admin@CORP.COM" -m "Password:" \
  --after "klist" \
  --after "echo 'Kerberos ticket refreshed'" \
  krb

# SSH + notify when disconnected
passauto add -c "ssh deploy@prod" -m "password:" \
  --after "echo 'Disconnected from prod at $(date)'" \
  prod
```

## System Administration

```bash
# Sudo
passauto add -c "sudo apt upgrade -y" -m "password" apt-upgrade

# Switch user
passauto add -c "su - postgres" -m "password:" su-postgres

# Kerberos
passauto add -c "kinit admin@CORP.EXAMPLE.COM" -m "Password for" krb
```

## Containers & CI

```bash
# Docker registry login
passauto add -c "docker login registry.example.com -u ci" -m "password:" docker-reg

# Podman login
passauto add -c "podman login ghcr.io -u bot" -m "password:" ghcr

# Helm registry
passauto add -c "helm registry login registry.example.com -u ci" -m "password:" helm-reg
```

## File Transfer

```bash
# SFTP
passauto add -c "sftp deploy@files.example.com" -m "password:" sftp-prod

# SCP (when PasswordAuthentication is enabled)
passauto add -c "scp backup.tar.gz admin@backup-server:/backups/" -m "password:" scp-backup
```

## Git (HTTPS)

```bash
# Git push over HTTPS (credential prompt)
passauto add -c "git push origin main" -m "Password for" git-push

# Git clone private repo
passauto add -c "git clone https://github.com/org/private-repo.git" -m "Password for" git-clone
```

## TOTP / Two-Factor Authentication

```bash
# TOTP-only (VPN token, MFA prompt)
passauto add -c "vpn-connect" -m "Verification code:" vpn --totp

# Password + TOTP (two-step login)
passauto add -c "ssh admin@secure-server" -m "password:" \
  --totp-match "Verification code:" secure-ssh

# AWS CLI MFA (token code prompt)
passauto add -c "aws sts get-session-token" -m "Enter MFA code:" aws-mfa --totp

# Run — both password and TOTP auto-filled
passauto secure-ssh
```

## Advanced Patterns

### Multiple Prompts

A profile can match multiple patterns (all share the same secret):

```bash
passauto add -c "ssh user@server" \
  -m "password:" \
  -m "Password:" \
  myserver
```

### Dry-Run

Preview what passauto will do without executing:

```bash
passauto myserver --dry-run
# Output:
# Command: ssh user@server
# Timeout: 30s
# Patterns:
#   - (?i)password:
```

### Quiet Mode (for scripts)

```bash
# No PTY output — just auto-fill and exit
passauto mydb -q --script queries.sql > result.txt
```

### Timeout Control

```bash
# Long-running session (5 min timeout)
passauto add -c "psql -h slow-db.example.com -U admin" -m "password" -t 5m psql-slow
```

## Backup & Migration

```bash
# Export profiles (no secrets) for sharing
passauto export team-profiles.json

# Import on another machine (merge, skip conflicts)
passauto import team-profiles.json

# Import with overwrite
passauto import team-profiles.json --force

# Full backup (key + encrypted data)
passauto backup /mnt/usb/passauto-backup

# Restore on new machine
passauto restore /mnt/usb/passauto-backup
```
