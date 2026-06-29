# passauto Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a cross-platform Go CLI that wraps commands in a PTY, matches output against regex patterns, and responds with secrets decrypted from an AES-256-GCM encrypted store keyed by SSH private key.

**Architecture:** Cobra CLI dispatches to internal packages: `crypto` (HKDF key derivation + AES-GCM), `store` (encrypted JSON secret storage), `config` (YAML profiles/patterns), and `engine` (PTY spawn + pattern matcher). Platform-specific PTY via build tags (`creack/pty` on Unix, `conpty` on Windows).

**Tech Stack:** Go 1.22+, cobra, creack/pty, conpty, golang.org/x/crypto (ssh, hkdf), gopkg.in/yaml.v3, golang.org/x/term

---

## File Structure

```
passauto/
├── main.go                         # Entry point, calls cmd.Execute()
├── go.mod                          # Module definition
├── cmd/
│   ├── root.go                     # Cobra root command, profile dispatch
│   ├── init_cmd.go                 # passauto init
│   ├── add.go                      # passauto add <name>
│   ├── list.go                     # passauto list
│   ├── remove.go                   # passauto remove <name>
│   └── run.go                      # passauto run <command...>
├── internal/
│   ├── crypto/
│   │   ├── crypto.go               # DeriveKey, Encrypt, Decrypt
│   │   └── crypto_test.go          # Round-trip, determinism, invalid key
│   ├── store/
│   │   ├── store.go                # Load, Save, Get, Put, Remove, List
│   │   └── store_test.go           # CRUD operations on temp files
│   ├── config/
│   │   ├── config.go               # Load, Validate, ResolveSecrets
│   │   └── config_test.go          # Parse, validation, template resolution
│   ├── engine/
│   │   ├── matcher.go              # PatternMatcher with regex + response
│   │   ├── matcher_test.go         # Pattern matching unit tests
│   │   ├── engine.go               # Engine interface + Run logic
│   │   ├── pty_unix.go             # Unix PTY implementation (build tag)
│   │   ├── pty_windows.go          # Windows ConPTY implementation (build tag)
│   │   └── engine_test.go          # Integration test with mock program
│   └── testutil/
│       └── mockprompt.go           # Test helper: program that prompts and reads
└── testdata/
    ├── test_key_ed25519            # Unencrypted test SSH key (for tests only)
    └── test_key_ed25519_enc        # Passphrase-protected test key (for tests only)
```

---

### Task 1: Project Scaffolding

**Files:**
- Create: `main.go`
- Create: `go.mod`
- Create: `cmd/root.go`

- [ ] **Step 1: Initialize Go module**

Run:
```bash
cd C:/workspace/ai-incubation/projects/passauto
go mod init passauto
```

Expected: `go.mod` created with `module passauto`

- [ ] **Step 2: Create main.go**

```go
package main

import "passauto/cmd"

func main() {
	cmd.Execute()
}
```

- [ ] **Step 3: Create cmd/root.go with cobra root command**

```go
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "passauto",
	Short: "Automated interactive prompt responder",
	Long:  "Wraps commands in a PTY, matches output patterns, and responds with decrypted secrets.",
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
```

- [ ] **Step 4: Install cobra dependency**

Run:
```bash
go get github.com/spf13/cobra@latest
go mod tidy
```

Expected: `go.sum` created, cobra added to `go.mod`

- [ ] **Step 5: Verify it compiles**

Run:
```bash
go build ./...
```

Expected: No errors, binary produced

- [ ] **Step 6: Commit**

```bash
git add main.go go.mod go.sum cmd/root.go
git commit -m "feat: scaffold passauto project with cobra CLI"
```

---

### Task 2: Crypto Module — Key Derivation

**Files:**
- Create: `internal/crypto/crypto.go`
- Create: `internal/crypto/crypto_test.go`
- Create: `testdata/test_key_ed25519`

- [ ] **Step 1: Generate a test SSH key for unit tests**

Run:
```bash
mkdir -p testdata
ssh-keygen -t ed25519 -f testdata/test_key_ed25519 -N "" -C "passauto-test"
```

Expected: `testdata/test_key_ed25519` (private) and `testdata/test_key_ed25519.pub` (public) created

- [ ] **Step 2: Write failing test for DeriveKey**

Create `internal/crypto/crypto_test.go`:

```go
package crypto

import (
	"path/filepath"
	"runtime"
	"testing"
)

func testdataPath(name string) string {
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(filename), "..", "..", "testdata", name)
}

func TestDeriveKey_Deterministic(t *testing.T) {
	keyPath := testdataPath("test_key_ed25519")

	key1, err := DeriveKey(keyPath, nil)
	if err != nil {
		t.Fatalf("DeriveKey failed: %v", err)
	}
	if len(key1) != 32 {
		t.Fatalf("expected 32-byte key, got %d", len(key1))
	}

	key2, err := DeriveKey(keyPath, nil)
	if err != nil {
		t.Fatalf("DeriveKey second call failed: %v", err)
	}

	if string(key1) != string(key2) {
		t.Fatal("DeriveKey not deterministic: two calls produced different keys")
	}
}

func TestDeriveKey_InvalidPath(t *testing.T) {
	_, err := DeriveKey("/nonexistent/path", nil)
	if err == nil {
		t.Fatal("expected error for invalid path")
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

Run:
```bash
go test ./internal/crypto/ -v
```

Expected: Compilation error — `DeriveKey` not defined

- [ ] **Step 4: Implement DeriveKey**

Create `internal/crypto/crypto.go`:

```go
package crypto

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/pem"
	"fmt"
	"io"
	"os"

	"golang.org/x/crypto/hkdf"
	"golang.org/x/crypto/ssh"
)

var (
	hkdfSalt = []byte("passauto-salt-v1")
	hkdfInfo = []byte("passauto-v1")
)

func DeriveKey(sshKeyPath string, passphrase []byte) ([]byte, error) {
	keyData, err := os.ReadFile(sshKeyPath)
	if err != nil {
		return nil, fmt.Errorf("reading SSH key: %w", err)
	}

	rawBytes, err := extractPrivateKeyBytes(keyData, passphrase)
	if err != nil {
		return nil, fmt.Errorf("parsing SSH key: %w", err)
	}

	hkdfReader := hkdf.New(sha256.New, rawBytes, hkdfSalt, hkdfInfo)
	derivedKey := make([]byte, 32)
	if _, err := io.ReadFull(hkdfReader, derivedKey); err != nil {
		return nil, fmt.Errorf("HKDF expansion: %w", err)
	}

	return derivedKey, nil
}

func extractPrivateKeyBytes(pemData, passphrase []byte) ([]byte, error) {
	var rawKey interface{}
	var err error

	if passphrase != nil {
		rawKey, err = ssh.ParseRawPrivateKeyWithPassphrase(pemData, passphrase)
	} else {
		rawKey, err = ssh.ParseRawPrivateKey(pemData)
	}
	if err != nil {
		return nil, err
	}

	switch k := rawKey.(type) {
	case *ed25519.PrivateKey:
		return []byte(*k), nil
	case ed25519.PrivateKey:
		return []byte(k), nil
	default:
		block, _ := pem.Decode(pemData)
		if block == nil {
			return nil, fmt.Errorf("unsupported key type %T and no PEM block found", rawKey)
		}
		return block.Bytes, nil
	}
}
```

- [ ] **Step 5: Install dependencies and run test**

Run:
```bash
go get golang.org/x/crypto/ssh golang.org/x/crypto/hkdf
go test ./internal/crypto/ -v
```

Expected: Both tests PASS

- [ ] **Step 6: Commit**

```bash
git add internal/crypto/ testdata/
git commit -m "feat(crypto): implement SSH key-based HKDF key derivation"
```

---

### Task 3: Crypto Module — AES-GCM Encrypt/Decrypt

**Files:**
- Modify: `internal/crypto/crypto.go`
- Modify: `internal/crypto/crypto_test.go`

- [ ] **Step 1: Write failing tests for Encrypt and Decrypt**

Append to `internal/crypto/crypto_test.go`:

```go
func TestEncryptDecrypt_RoundTrip(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}

	plaintext := []byte("my-secret-password")

	ciphertext, err := Encrypt(key, plaintext)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	if string(ciphertext) == string(plaintext) {
		t.Fatal("ciphertext should differ from plaintext")
	}

	decrypted, err := Decrypt(key, ciphertext)
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}

	if string(decrypted) != string(plaintext) {
		t.Fatalf("expected %q, got %q", plaintext, decrypted)
	}
}

func TestDecrypt_WrongKey(t *testing.T) {
	key1 := make([]byte, 32)
	key2 := make([]byte, 32)
	key2[0] = 0xFF

	ciphertext, err := Encrypt(key1, []byte("secret"))
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	_, err = Decrypt(key2, ciphertext)
	if err == nil {
		t.Fatal("expected error decrypting with wrong key")
	}
}

func TestEncrypt_DifferentNonce(t *testing.T) {
	key := make([]byte, 32)
	plaintext := []byte("same-input")

	c1, _ := Encrypt(key, plaintext)
	c2, _ := Encrypt(key, plaintext)

	if string(c1) == string(c2) {
		t.Fatal("two encryptions of same plaintext should produce different ciphertext (random nonce)")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run:
```bash
go test ./internal/crypto/ -v
```

Expected: Compilation error — `Encrypt`, `Decrypt` not defined

- [ ] **Step 3: Implement Encrypt and Decrypt**

Append to `internal/crypto/crypto.go`:

```go
import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
)

func Encrypt(key, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("creating AES cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("creating GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("generating nonce: %w", err)
	}

	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

func Decrypt(key, ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("creating AES cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("creating GCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertextBody := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertextBody, nil)
	if err != nil {
		return nil, fmt.Errorf("decrypting: %w", err)
	}

	return plaintext, nil
}
```

Note: Merge the new imports (`crypto/aes`, `crypto/cipher`, `crypto/rand`) into the existing import block at the top of `crypto.go`.

- [ ] **Step 4: Run tests**

Run:
```bash
go test ./internal/crypto/ -v
```

Expected: All 5 tests PASS

- [ ] **Step 5: Commit**

```bash
git add internal/crypto/
git commit -m "feat(crypto): implement AES-256-GCM encrypt/decrypt"
```

---

### Task 4: Secret Store

**Files:**
- Create: `internal/store/store.go`
- Create: `internal/store/store_test.go`

- [ ] **Step 1: Write failing tests for Store**

Create `internal/store/store_test.go`:

```go
package store

import (
	"os"
	"path/filepath"
	"testing"
)

func tempStorePath(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	return filepath.Join(dir, "secrets.enc")
}

func testKey() []byte {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	return key
}

func TestStore_PutAndGet(t *testing.T) {
	path := tempStorePath(t)
	key := testKey()

	s, err := Open(path, key)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}

	if err := s.Put("my-secret", []byte("hunter2")); err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	val, err := s.Get("my-secret")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if string(val) != "hunter2" {
		t.Fatalf("expected 'hunter2', got %q", val)
	}
}

func TestStore_List(t *testing.T) {
	path := tempStorePath(t)
	key := testKey()

	s, _ := Open(path, key)
	s.Put("alpha", []byte("a"))
	s.Put("beta", []byte("b"))

	names := s.List()
	if len(names) != 2 {
		t.Fatalf("expected 2 names, got %d", len(names))
	}
}

func TestStore_Remove(t *testing.T) {
	path := tempStorePath(t)
	key := testKey()

	s, _ := Open(path, key)
	s.Put("temp", []byte("data"))
	if err := s.Remove("temp"); err != nil {
		t.Fatalf("Remove failed: %v", err)
	}

	_, err := s.Get("temp")
	if err == nil {
		t.Fatal("expected error getting removed secret")
	}
}

func TestStore_Persistence(t *testing.T) {
	path := tempStorePath(t)
	key := testKey()

	s1, _ := Open(path, key)
	s1.Put("persist", []byte("value"))

	s2, err := Open(path, key)
	if err != nil {
		t.Fatalf("Re-open failed: %v", err)
	}

	val, err := s2.Get("persist")
	if err != nil {
		t.Fatalf("Get after re-open failed: %v", err)
	}
	if string(val) != "value" {
		t.Fatalf("expected 'value', got %q", val)
	}
}

func TestStore_FilePermissions(t *testing.T) {
	if os.Getenv("GOOS") == "windows" {
		t.Skip("file permissions not applicable on Windows")
	}

	path := tempStorePath(t)
	key := testKey()

	s, _ := Open(path, key)
	s.Put("check", []byte("perm"))

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat failed: %v", err)
	}
	perm := info.Mode().Perm()
	if perm != 0600 {
		t.Fatalf("expected 0600 permissions, got %o", perm)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run:
```bash
go test ./internal/store/ -v
```

Expected: Compilation error — `Open` not defined

- [ ] **Step 3: Implement the store**

Create `internal/store/store.go`:

```go
package store

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"passauto/internal/crypto"
)

type Store struct {
	path    string
	key     []byte
	secrets map[string]string
}

func Open(path string, key []byte) (*Store, error) {
	s := &Store{
		path:    path,
		key:     key,
		secrets: make(map[string]string),
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return s, nil
		}
		return nil, fmt.Errorf("reading store: %w", err)
	}

	if err := json.Unmarshal(data, &s.secrets); err != nil {
		return nil, fmt.Errorf("parsing store: %w", err)
	}

	return s, nil
}

func (s *Store) Put(name string, plaintext []byte) error {
	ciphertext, err := crypto.Encrypt(s.key, plaintext)
	if err != nil {
		return fmt.Errorf("encrypting secret: %w", err)
	}

	s.secrets[name] = base64.StdEncoding.EncodeToString(ciphertext)
	return s.save()
}

func (s *Store) Get(name string) ([]byte, error) {
	encoded, ok := s.secrets[name]
	if !ok {
		return nil, fmt.Errorf("secret %q not found", name)
	}

	ciphertext, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("decoding secret: %w", err)
	}

	return crypto.Decrypt(s.key, ciphertext)
}

func (s *Store) Remove(name string) error {
	if _, ok := s.secrets[name]; !ok {
		return fmt.Errorf("secret %q not found", name)
	}
	delete(s.secrets, name)
	return s.save()
}

func (s *Store) List() []string {
	names := make([]string, 0, len(s.secrets))
	for name := range s.secrets {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func (s *Store) save() error {
	data, err := json.MarshalIndent(s.secrets, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling store: %w", err)
	}

	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("creating store directory: %w", err)
	}

	if err := os.WriteFile(s.path, data, 0600); err != nil {
		return fmt.Errorf("writing store: %w", err)
	}

	return nil
}
```

- [ ] **Step 4: Run tests**

Run:
```bash
go test ./internal/store/ -v
```

Expected: All 5 tests PASS

- [ ] **Step 5: Commit**

```bash
git add internal/store/
git commit -m "feat(store): implement encrypted secret store with CRUD operations"
```

---

### Task 5: Config Module

**Files:**
- Create: `internal/config/config.go`
- Create: `internal/config/config_test.go`

- [ ] **Step 1: Write failing tests for config loading**

Create `internal/config/config_test.go`:

```go
package config

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTestConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("writing test config: %v", err)
	}
	return path
}

func TestLoad_ValidConfig(t *testing.T) {
	path := writeTestConfig(t, `
ssh_key: ~/.ssh/id_ed25519
defaults:
  patterns:
    - match: "(?i)password:"
      respond: "{{my-pass}}"
      hidden: true
profiles:
  mwinit:
    command: "mwinit -s -o"
    patterns:
      - match: "(?i)password:"
        respond: "{{my-pass}}"
        hidden: true
      - match: "\\(yes/no\\)"
        respond: "yes"
    timeout: 30s
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.SSHKey != "~/.ssh/id_ed25519" {
		t.Fatalf("unexpected ssh_key: %s", cfg.SSHKey)
	}
	if len(cfg.Defaults.Patterns) != 1 {
		t.Fatalf("expected 1 default pattern, got %d", len(cfg.Defaults.Patterns))
	}
	if len(cfg.Profiles) != 1 {
		t.Fatalf("expected 1 profile, got %d", len(cfg.Profiles))
	}
	if cfg.Profiles["mwinit"].Command != "mwinit -s -o" {
		t.Fatalf("unexpected command: %s", cfg.Profiles["mwinit"].Command)
	}
	if len(cfg.Profiles["mwinit"].Patterns) != 2 {
		t.Fatalf("expected 2 patterns in mwinit profile, got %d", len(cfg.Profiles["mwinit"].Patterns))
	}
}

func TestLoad_InvalidRegex(t *testing.T) {
	path := writeTestConfig(t, `
ssh_key: ~/.ssh/id_ed25519
defaults:
  patterns:
    - match: "[invalid"
      respond: "x"
`)

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for invalid regex")
	}
}

func TestLoad_ReservedProfileName(t *testing.T) {
	path := writeTestConfig(t, `
ssh_key: ~/.ssh/id_ed25519
profiles:
  init:
    command: "something"
    patterns:
      - match: "x"
        respond: "y"
`)

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for reserved profile name 'init'")
	}
}

func TestResolveSecrets_Valid(t *testing.T) {
	patterns := []Pattern{
		{Match: "password:", Respond: "{{my-pass}}", Hidden: true},
		{Match: "confirm", Respond: "yes", Hidden: false},
	}

	secrets := map[string][]byte{
		"my-pass": []byte("hunter2"),
	}

	resolved, err := ResolveSecrets(patterns, secrets)
	if err != nil {
		t.Fatalf("ResolveSecrets failed: %v", err)
	}

	if resolved[0].Respond != "hunter2" {
		t.Fatalf("expected 'hunter2', got %q", resolved[0].Respond)
	}
	if resolved[1].Respond != "yes" {
		t.Fatalf("expected 'yes', got %q", resolved[1].Respond)
	}
}

func TestResolveSecrets_MissingSecret(t *testing.T) {
	patterns := []Pattern{
		{Match: "password:", Respond: "{{missing}}", Hidden: true},
	}

	_, err := ResolveSecrets(patterns, map[string][]byte{})
	if err == nil {
		t.Fatal("expected error for missing secret")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run:
```bash
go test ./internal/config/ -v
```

Expected: Compilation error — types and functions not defined

- [ ] **Step 3: Implement config module**

Create `internal/config/config.go`:

```go
package config

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

var reservedNames = map[string]bool{
	"init":   true,
	"add":    true,
	"list":   true,
	"remove": true,
	"run":    true,
	"help":   true,
}

type Config struct {
	SSHKey   string             `yaml:"ssh_key"`
	Defaults DefaultConfig      `yaml:"defaults"`
	Profiles map[string]Profile `yaml:"profiles"`
}

type DefaultConfig struct {
	Patterns []Pattern `yaml:"patterns"`
}

type Pattern struct {
	Match   string `yaml:"match"`
	Respond string `yaml:"respond"`
	Hidden  bool   `yaml:"hidden"`
}

type Profile struct {
	Command  string        `yaml:"command"`
	Patterns []Pattern     `yaml:"patterns"`
	Timeout  time.Duration `yaml:"timeout"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	if err := validate(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func validate(cfg *Config) error {
	for _, p := range cfg.Defaults.Patterns {
		if _, err := regexp.Compile(p.Match); err != nil {
			return fmt.Errorf("invalid regex in defaults %q: %w", p.Match, err)
		}
	}

	for name, profile := range cfg.Profiles {
		if reservedNames[name] {
			return fmt.Errorf("profile name %q conflicts with a subcommand", name)
		}
		for _, p := range profile.Patterns {
			if _, err := regexp.Compile(p.Match); err != nil {
				return fmt.Errorf("invalid regex in profile %q pattern %q: %w", name, p.Match, err)
			}
		}
	}

	return nil
}

func ResolveSecrets(patterns []Pattern, secrets map[string][]byte) ([]Pattern, error) {
	resolved := make([]Pattern, len(patterns))
	copy(resolved, patterns)

	for i, p := range resolved {
		if strings.HasPrefix(p.Respond, "{{") && strings.HasSuffix(p.Respond, "}}") {
			name := strings.TrimPrefix(strings.TrimSuffix(p.Respond, "}}"), "{{")
			val, ok := secrets[name]
			if !ok {
				return nil, fmt.Errorf("secret %q not found in store", name)
			}
			resolved[i].Respond = string(val)
		}
	}

	return resolved, nil
}
```

- [ ] **Step 4: Install yaml dependency and run tests**

Run:
```bash
go get gopkg.in/yaml.v3
go test ./internal/config/ -v
```

Expected: All 5 tests PASS

- [ ] **Step 5: Commit**

```bash
git add internal/config/
git commit -m "feat(config): implement YAML config loading with validation and secret resolution"
```

---

### Task 6: Pattern Matcher

**Files:**
- Create: `internal/engine/matcher.go`
- Create: `internal/engine/matcher_test.go`

- [ ] **Step 1: Write failing tests for PatternMatcher**

Create `internal/engine/matcher_test.go`:

```go
package engine

import (
	"testing"

	"passauto/internal/config"
)

func TestMatcher_SimpleMatch(t *testing.T) {
	patterns := []config.Pattern{
		{Match: "(?i)password:", Respond: "secret123", Hidden: true},
	}

	m, err := NewMatcher(patterns)
	if err != nil {
		t.Fatalf("NewMatcher failed: %v", err)
	}

	resp := m.Check("Enter password:")
	if resp == nil {
		t.Fatal("expected a match")
	}
	if resp.Response != "secret123" {
		t.Fatalf("expected 'secret123', got %q", resp.Response)
	}
	if !resp.Hidden {
		t.Fatal("expected hidden=true")
	}
}

func TestMatcher_NoMatch(t *testing.T) {
	patterns := []config.Pattern{
		{Match: "(?i)password:", Respond: "secret123", Hidden: true},
	}

	m, _ := NewMatcher(patterns)

	resp := m.Check("Loading configuration...")
	if resp != nil {
		t.Fatal("expected no match")
	}
}

func TestMatcher_FirstMatchWins(t *testing.T) {
	patterns := []config.Pattern{
		{Match: "password", Respond: "first", Hidden: true},
		{Match: "pass", Respond: "second", Hidden: false},
	}

	m, _ := NewMatcher(patterns)

	resp := m.Check("enter password now")
	if resp == nil {
		t.Fatal("expected a match")
	}
	if resp.Response != "first" {
		t.Fatalf("expected first match to win, got %q", resp.Response)
	}
}

func TestMatcher_InvalidRegex(t *testing.T) {
	patterns := []config.Pattern{
		{Match: "[broken", Respond: "x", Hidden: false},
	}

	_, err := NewMatcher(patterns)
	if err == nil {
		t.Fatal("expected error for invalid regex")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run:
```bash
go test ./internal/engine/ -v
```

Expected: Compilation error — `NewMatcher` not defined

- [ ] **Step 3: Implement PatternMatcher**

Create `internal/engine/matcher.go`:

```go
package engine

import (
	"fmt"
	"regexp"

	"passauto/internal/config"
)

type MatchResult struct {
	Response string
	Hidden   bool
}

type compiledPattern struct {
	regex    *regexp.Regexp
	response string
	hidden   bool
}

type Matcher struct {
	patterns []compiledPattern
}

func NewMatcher(patterns []config.Pattern) (*Matcher, error) {
	compiled := make([]compiledPattern, len(patterns))
	for i, p := range patterns {
		re, err := regexp.Compile(p.Match)
		if err != nil {
			return nil, fmt.Errorf("compiling pattern %q: %w", p.Match, err)
		}
		compiled[i] = compiledPattern{
			regex:    re,
			response: p.Respond,
			hidden:   p.Hidden,
		}
	}
	return &Matcher{patterns: compiled}, nil
}

func (m *Matcher) Check(line string) *MatchResult {
	for _, p := range m.patterns {
		if p.regex.MatchString(line) {
			return &MatchResult{
				Response: p.response,
				Hidden:   p.hidden,
			}
		}
	}
	return nil
}
```

- [ ] **Step 4: Run tests**

Run:
```bash
go test ./internal/engine/ -v
```

Expected: All 4 tests PASS

- [ ] **Step 5: Commit**

```bash
git add internal/engine/matcher.go internal/engine/matcher_test.go
git commit -m "feat(engine): implement regex pattern matcher"
```

---

### Task 7: PTY Engine — Unix Implementation

**Files:**
- Create: `internal/engine/engine.go`
- Create: `internal/engine/pty_unix.go`

- [ ] **Step 1: Create engine interface and run logic**

Create `internal/engine/engine.go`:

```go
package engine

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"passauto/internal/config"
)

type Options struct {
	Command []string
	Patterns []config.Pattern
	Timeout  time.Duration
	Stdin    io.Reader
	Stdout   io.Writer
	Stderr   io.Writer
}

func Run(opts Options) (int, error) {
	if opts.Timeout == 0 {
		opts.Timeout = 30 * time.Second
	}
	if opts.Stdin == nil {
		opts.Stdin = os.Stdin
	}
	if opts.Stdout == nil {
		opts.Stdout = os.Stdout
	}
	if opts.Stderr == nil {
		opts.Stderr = os.Stderr
	}

	matcher, err := NewMatcher(opts.Patterns)
	if err != nil {
		return 1, fmt.Errorf("creating matcher: %w", err)
	}

	return runWithPTY(opts, matcher)
}
```

- [ ] **Step 2: Create Unix PTY implementation**

Create `internal/engine/pty_unix.go`:

```go
//go:build !windows

package engine

import (
	"bufio"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/creack/pty"
	"golang.org/x/term"
)

func runWithPTY(opts Options, matcher *Matcher) (int, error) {
	cmd := exec.Command(opts.Command[0], opts.Command[1:]...)

	ptmx, err := pty.Start(cmd)
	if err != nil {
		return 1, fmt.Errorf("starting PTY: %w", err)
	}
	defer ptmx.Close()

	// Handle terminal resize
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGWINCH)
	go func() {
		for range sigCh {
			pty.InheritSize(os.Stdin, ptmx)
		}
	}()
	sigCh <- syscall.SIGWINCH // Initial resize

	// Set stdin to raw mode if it's a terminal
	if fd := int(os.Stdin.Fd()); term.IsTerminal(fd) {
		oldState, err := term.MakeRaw(fd)
		if err == nil {
			defer term.Restore(fd, oldState)
		}
	}

	var wg sync.WaitGroup

	// Forward user input to PTY
	wg.Add(1)
	go func() {
		defer wg.Done()
		io.Copy(ptmx, opts.Stdin)
	}()

	// Read PTY output, match patterns, forward to stdout
	wg.Add(1)
	go func() {
		defer wg.Done()

		lineBuf := ""
		buf := make([]byte, 4096)
		timer := time.NewTimer(100 * time.Millisecond)
		timer.Stop()

		reader := bufio.NewReader(ptmx)

		for {
			n, err := reader.Read(buf)
			if n > 0 {
				chunk := string(buf[:n])
				opts.Stdout.Write(buf[:n])

				lineBuf += chunk

				// Check for newlines or use timer for partial lines
				for {
					idx := findNewline(lineBuf)
					if idx < 0 {
						timer.Reset(100 * time.Millisecond)
						break
					}
					line := lineBuf[:idx+1]
					lineBuf = lineBuf[idx+1:]
					checkAndRespond(matcher, line, ptmx)
				}
			}
			if err != nil {
				// Check remaining buffer
				if lineBuf != "" {
					checkAndRespond(matcher, lineBuf, ptmx)
				}
				break
			}

			select {
			case <-timer.C:
				if lineBuf != "" {
					checkAndRespond(matcher, lineBuf, ptmx)
					lineBuf = ""
				}
			default:
			}
		}
	}()

	err = cmd.Wait()
	signal.Stop(sigCh)
	close(sigCh)

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return 1, fmt.Errorf("waiting for command: %w", err)
		}
	}

	return exitCode, nil
}

func checkAndRespond(matcher *Matcher, line string, ptmx *os.File) {
	result := matcher.Check(line)
	if result != nil {
		ptmx.Write([]byte(result.Response + "\n"))
	}
}

func findNewline(s string) int {
	for i, c := range s {
		if c == '\n' || c == '\r' {
			return i
		}
	}
	return -1
}
```

- [ ] **Step 3: Install PTY dependencies**

Run:
```bash
go get github.com/creack/pty
go get golang.org/x/term
```

- [ ] **Step 4: Verify compilation on Unix (or skip on Windows)**

Run:
```bash
GOOS=linux go build ./internal/engine/
```

Expected: No errors (if on Windows, cross-compilation check only)

- [ ] **Step 5: Commit**

```bash
git add internal/engine/engine.go internal/engine/pty_unix.go
git commit -m "feat(engine): implement Unix PTY engine with pattern matching"
```

---

### Task 8: PTY Engine — Windows Implementation

**Files:**
- Create: `internal/engine/pty_windows.go`

- [ ] **Step 1: Implement Windows ConPTY variant**

Create `internal/engine/pty_windows.go`:

```go
//go:build windows

package engine

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/UserExistsError/conpty"
)

func runWithPTY(opts Options, matcher *Matcher) (int, error) {
	cmdLine := opts.Command[0]
	for _, arg := range opts.Command[1:] {
		cmdLine += " " + arg
	}

	cpty, err := conpty.Start(cmdLine)
	if err != nil {
		return 1, fmt.Errorf("starting ConPTY: %w", err)
	}
	defer cpty.Close()

	var wg sync.WaitGroup

	// Forward user input to ConPTY
	wg.Add(1)
	go func() {
		defer wg.Done()
		io.Copy(cpty, opts.Stdin)
	}()

	// Read ConPTY output, match patterns, forward to stdout
	wg.Add(1)
	go func() {
		defer wg.Done()

		lineBuf := ""
		buf := make([]byte, 4096)
		timer := time.NewTimer(100 * time.Millisecond)
		timer.Stop()

		reader := bufio.NewReader(cpty)

		for {
			n, err := reader.Read(buf)
			if n > 0 {
				chunk := string(buf[:n])
				opts.Stdout.Write(buf[:n])

				lineBuf += chunk

				for {
					idx := findNewline(lineBuf)
					if idx < 0 {
						timer.Reset(100 * time.Millisecond)
						break
					}
					line := lineBuf[:idx+1]
					lineBuf = lineBuf[idx+1:]
					checkAndRespondWriter(matcher, line, cpty)
				}
			}
			if err != nil {
				if lineBuf != "" {
					checkAndRespondWriter(matcher, lineBuf, cpty)
				}
				break
			}

			select {
			case <-timer.C:
				if lineBuf != "" {
					checkAndRespondWriter(matcher, lineBuf, cpty)
					lineBuf = ""
				}
			default:
			}
		}
	}()

	exitCode, err := cpty.Wait(0)
	if err != nil {
		return 1, fmt.Errorf("waiting for command: %w", err)
	}

	return int(exitCode), nil
}

func checkAndRespondWriter(matcher *Matcher, line string, w io.Writer) {
	result := matcher.Check(line)
	if result != nil {
		w.Write([]byte(result.Response + "\n"))
	}
}
```

- [ ] **Step 2: Install conpty dependency**

Run:
```bash
go get github.com/UserExistsError/conpty
```

- [ ] **Step 3: Verify Windows compilation**

Run:
```bash
GOOS=windows go build ./internal/engine/
```

Expected: No errors

- [ ] **Step 4: Commit**

```bash
git add internal/engine/pty_windows.go
git commit -m "feat(engine): implement Windows ConPTY engine"
```

---

### Task 9: PTY Engine — Integration Test

**Files:**
- Create: `internal/testutil/mockprompt.go`
- Create: `internal/engine/engine_test.go`

- [ ] **Step 1: Create mock prompt program for testing**

Create `internal/testutil/mockprompt.go`:

```go
//go:build ignore

package main

import (
	"bufio"
	"fmt"
	"os"
)

func main() {
	scanner := bufio.NewScanner(os.Stdin)

	fmt.Print("Password: ")
	if scanner.Scan() {
		pw := scanner.Text()
		if pw == "correct-password" {
			fmt.Println("Access granted")
		} else {
			fmt.Println("Access denied")
			os.Exit(1)
		}
	}

	fmt.Print("Continue? (yes/no): ")
	if scanner.Scan() {
		answer := scanner.Text()
		if answer == "yes" {
			fmt.Println("Done!")
		} else {
			fmt.Println("Aborted")
			os.Exit(1)
		}
	}
}
```

- [ ] **Step 2: Write integration test**

Create `internal/engine/engine_test.go`:

```go
//go:build !windows

package engine

import (
	"bytes"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"passauto/internal/config"
)

func buildMockPrompt(t *testing.T) string {
	t.Helper()
	_, filename, _, _ := runtime.Caller(0)
	srcPath := filepath.Join(filepath.Dir(filename), "..", "testutil", "mockprompt.go")
	binPath := filepath.Join(t.TempDir(), "mockprompt")

	cmd := exec.Command("go", "build", "-o", binPath, srcPath)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("building mockprompt: %v\n%s", err, output)
	}
	return binPath
}

func TestEngine_MultiPrompt(t *testing.T) {
	bin := buildMockPrompt(t)

	patterns := []config.Pattern{
		{Match: "(?i)password:", Respond: "correct-password", Hidden: true},
		{Match: "\\(yes/no\\)", Respond: "yes", Hidden: false},
	}

	var stdout bytes.Buffer

	exitCode, err := Run(Options{
		Command:  []string{bin},
		Patterns: patterns,
		Timeout:  5 * time.Second,
		Stdin:    &bytes.Buffer{},
		Stdout:   &stdout,
	})

	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d\nOutput: %s", exitCode, stdout.String())
	}

	output := stdout.String()
	if !bytes.Contains([]byte(output), []byte("Access granted")) {
		t.Fatalf("expected 'Access granted' in output, got: %s", output)
	}
	if !bytes.Contains([]byte(output), []byte("Done!")) {
		t.Fatalf("expected 'Done!' in output, got: %s", output)
	}
}
```

- [ ] **Step 3: Run integration test**

Run:
```bash
go test ./internal/engine/ -v -run TestEngine
```

Expected: PASS (on Linux/macOS; skip on Windows build)

- [ ] **Step 4: Commit**

```bash
git add internal/testutil/ internal/engine/engine_test.go
git commit -m "test(engine): add integration test with multi-prompt mock program"
```

---

### Task 10: CLI — init Command

**Files:**
- Create: `cmd/init_cmd.go`

- [ ] **Step 1: Implement init command**

Create `cmd/init_cmd.go`:

```go
package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"passauto/internal/crypto"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize passauto (first-time setup)",
	RunE:  runInit,
}

func init() {
	rootCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command, args []string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("getting home directory: %w", err)
	}

	passautoDir := filepath.Join(home, ".passauto")
	configPath := filepath.Join(passautoDir, "config.yaml")

	if _, err := os.Stat(configPath); err == nil {
		fmt.Println("passauto is already initialized. Config at:", configPath)
		return nil
	}

	sshKey := findSSHKey(home)
	if sshKey == "" {
		return fmt.Errorf("no SSH key found in ~/.ssh/. Generate one with: ssh-keygen -t ed25519")
	}

	fmt.Printf("Using SSH key: %s\n", sshKey)

	// Verify we can read the key (may need passphrase)
	var passphrase []byte
	_, err = crypto.DeriveKey(sshKey, nil)
	if err != nil {
		fmt.Print("Enter SSH key passphrase: ")
		passphrase, err = term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Println()
		if err != nil {
			return fmt.Errorf("reading passphrase: %w", err)
		}

		_, err = crypto.DeriveKey(sshKey, passphrase)
		if err != nil {
			return fmt.Errorf("cannot derive key from SSH key: %w", err)
		}
	}

	if err := os.MkdirAll(passautoDir, 0700); err != nil {
		return fmt.Errorf("creating ~/.passauto: %w", err)
	}

	defaultConfig := fmt.Sprintf(`ssh_key: %s

defaults:
  patterns:
    - match: "(?i)password:"
      respond: "{{default-password}}"
      hidden: true

profiles: {}
`, sshKey)

	if err := os.WriteFile(configPath, []byte(defaultConfig), 0600); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}

	fmt.Println("Initialized. Config at:", configPath)
	fmt.Println("Next: run 'passauto add <name>' to store a secret.")
	return nil
}

func findSSHKey(home string) string {
	sshDir := filepath.Join(home, ".ssh")
	candidates := []string{"id_ed25519", "id_rsa", "id_ecdsa"}

	for _, name := range candidates {
		path := filepath.Join(sshDir, name)
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}
```

- [ ] **Step 2: Verify compilation**

Run:
```bash
go build ./...
```

Expected: No errors

- [ ] **Step 3: Commit**

```bash
git add cmd/init_cmd.go
git commit -m "feat(cli): implement init command with SSH key detection"
```

---

### Task 11: CLI — add, list, remove Commands

**Files:**
- Create: `cmd/add.go`
- Create: `cmd/list.go`
- Create: `cmd/remove.go`

- [ ] **Step 1: Implement add command**

Create `cmd/add.go`:

```go
package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"passauto/internal/config"
	"passauto/internal/crypto"
	"passauto/internal/store"
)

var addCmd = &cobra.Command{
	Use:   "add <name>",
	Short: "Store a new secret",
	Args:  cobra.ExactArgs(1),
	RunE:  runAdd,
}

func init() {
	rootCmd.AddCommand(addCmd)
}

func runAdd(cmd *cobra.Command, args []string) error {
	name := args[0]

	key, err := loadEncryptionKey()
	if err != nil {
		return err
	}

	home, _ := os.UserHomeDir()
	storePath := filepath.Join(home, ".passauto", "secrets.enc")

	s, err := store.Open(storePath, key)
	if err != nil {
		return fmt.Errorf("opening store: %w", err)
	}

	fmt.Printf("Enter secret for %q: ", name)
	secret, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println()
	if err != nil {
		return fmt.Errorf("reading secret: %w", err)
	}

	if err := s.Put(name, secret); err != nil {
		return fmt.Errorf("storing secret: %w", err)
	}

	fmt.Printf("Stored %q (encrypted)\n", name)
	return nil
}

func loadEncryptionKey() ([]byte, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("getting home directory: %w", err)
	}

	configPath := filepath.Join(home, ".passauto", "config.yaml")
	cfg, err := config.Load(configPath)
	if err != nil {
		return nil, fmt.Errorf("loading config (have you run 'passauto init'?): %w", err)
	}

	sshKeyPath := cfg.SSHKey
	if sshKeyPath[:2] == "~/" {
		sshKeyPath = filepath.Join(home, sshKeyPath[2:])
	}

	key, err := crypto.DeriveKey(sshKeyPath, nil)
	if err != nil {
		fmt.Print("Enter SSH key passphrase: ")
		passphrase, readErr := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Println()
		if readErr != nil {
			return nil, fmt.Errorf("reading passphrase: %w", readErr)
		}
		key, err = crypto.DeriveKey(sshKeyPath, passphrase)
		if err != nil {
			return nil, fmt.Errorf("deriving key: %w", err)
		}
	}

	return key, nil
}
```

- [ ] **Step 2: Implement list command**

Create `cmd/list.go`:

```go
package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"passauto/internal/store"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List stored secret names",
	RunE:  runList,
}

func init() {
	rootCmd.AddCommand(listCmd)
}

func runList(cmd *cobra.Command, args []string) error {
	key, err := loadEncryptionKey()
	if err != nil {
		return err
	}

	home, _ := os.UserHomeDir()
	storePath := filepath.Join(home, ".passauto", "secrets.enc")

	s, err := store.Open(storePath, key)
	if err != nil {
		return fmt.Errorf("opening store: %w", err)
	}

	names := s.List()
	if len(names) == 0 {
		fmt.Println("No secrets stored. Use 'passauto add <name>' to add one.")
		return nil
	}

	for _, name := range names {
		fmt.Println(name)
	}
	return nil
}
```

- [ ] **Step 3: Implement remove command**

Create `cmd/remove.go`:

```go
package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"passauto/internal/store"
)

var removeCmd = &cobra.Command{
	Use:   "remove <name>",
	Short: "Delete a stored secret",
	Args:  cobra.ExactArgs(1),
	RunE:  runRemove,
}

func init() {
	rootCmd.AddCommand(removeCmd)
}

func runRemove(cmd *cobra.Command, args []string) error {
	name := args[0]

	key, err := loadEncryptionKey()
	if err != nil {
		return err
	}

	home, _ := os.UserHomeDir()
	storePath := filepath.Join(home, ".passauto", "secrets.enc")

	s, err := store.Open(storePath, key)
	if err != nil {
		return fmt.Errorf("opening store: %w", err)
	}

	if err := s.Remove(name); err != nil {
		return err
	}

	fmt.Printf("Removed %q\n", name)
	return nil
}
```

- [ ] **Step 4: Verify compilation**

Run:
```bash
go build ./...
```

Expected: No errors

- [ ] **Step 5: Commit**

```bash
git add cmd/add.go cmd/list.go cmd/remove.go
git commit -m "feat(cli): implement add, list, remove secret management commands"
```

---

### Task 12: CLI — run Command and Profile Dispatch

**Files:**
- Create: `cmd/run.go`
- Modify: `cmd/root.go`

- [ ] **Step 1: Implement run command**

Create `cmd/run.go`:

```go
package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"passauto/internal/config"
	"passauto/internal/engine"
	"passauto/internal/store"
)

var runCmd = &cobra.Command{
	Use:   "run [command...]",
	Short: "Run a command with automatic prompt handling",
	Args:  cobra.MinimumNArgs(1),
	RunE:  runRun,
}

func init() {
	rootCmd.AddCommand(runCmd)
}

func runRun(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	patterns := cfg.Defaults.Patterns
	timeout := 30 * time.Second

	return executeWithPatterns(args, patterns, timeout, cfg)
}

func runProfile(profileName string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	profile, ok := cfg.Profiles[profileName]
	if !ok {
		return fmt.Errorf("profile %q not found in config", profileName)
	}

	command := splitCommand(profile.Command)
	patterns := profile.Patterns
	timeout := profile.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	return executeWithPatterns(command, patterns, timeout, cfg)
}

func executeWithPatterns(command []string, patterns []config.Pattern, timeout time.Duration, cfg *config.Config) error {
	key, err := loadEncryptionKey()
	if err != nil {
		return err
	}

	home, _ := os.UserHomeDir()
	storePath := filepath.Join(home, ".passauto", "secrets.enc")

	s, err := store.Open(storePath, key)
	if err != nil {
		return fmt.Errorf("opening store: %w", err)
	}

	// Resolve secret references in patterns
	secrets := make(map[string][]byte)
	for _, p := range patterns {
		if name, ok := extractSecretRef(p.Respond); ok {
			val, err := s.Get(name)
			if err != nil {
				return fmt.Errorf("secret %q referenced in pattern but not found: %w", name, err)
			}
			secrets[name] = val
		}
	}

	resolved, err := config.ResolveSecrets(patterns, secrets)
	if err != nil {
		return err
	}

	exitCode, err := engine.Run(engine.Options{
		Command:  command,
		Patterns: resolved,
		Timeout:  timeout,
	})

	if err != nil {
		return err
	}

	os.Exit(exitCode)
	return nil
}

func loadConfig() (*config.Config, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("getting home directory: %w", err)
	}

	configPath := filepath.Join(home, ".passauto", "config.yaml")
	cfg, err := config.Load(configPath)
	if err != nil {
		return nil, fmt.Errorf("loading config (have you run 'passauto init'?): %w", err)
	}

	return cfg, nil
}

func extractSecretRef(respond string) (string, bool) {
	if len(respond) > 4 && respond[:2] == "{{" && respond[len(respond)-2:] == "}}" {
		return respond[2 : len(respond)-2], true
	}
	return "", false
}

func splitCommand(cmd string) []string {
	// Simple split by spaces — adequate for most use cases
	// For complex quoting, users can use the array form in profiles
	fields := []string{}
	current := ""
	inQuote := false
	quoteChar := byte(0)

	for i := 0; i < len(cmd); i++ {
		c := cmd[i]
		if inQuote {
			if c == quoteChar {
				inQuote = false
			} else {
				current += string(c)
			}
		} else if c == '"' || c == '\'' {
			inQuote = true
			quoteChar = c
		} else if c == ' ' {
			if current != "" {
				fields = append(fields, current)
				current = ""
			}
		} else {
			current += string(c)
		}
	}
	if current != "" {
		fields = append(fields, current)
	}
	return fields
}
```

- [ ] **Step 2: Verify imports in run.go**

Ensure the import block in `cmd/run.go` includes:
```go
import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"passauto/internal/config"
	"passauto/internal/engine"
	"passauto/internal/store"
)
```

- [ ] **Step 3: Add profile dispatch to root command**

Modify `cmd/root.go` to handle unknown subcommands as profile names:

```go
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "passauto",
	Short: "Automated interactive prompt responder",
	Long:  "Wraps commands in a PTY, matches output patterns, and responds with decrypted secrets.",
	Args:  cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return cmd.Help()
		}
		return runProfile(args[0])
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
```

- [ ] **Step 4: Verify compilation**

Run:
```bash
go build ./...
```

Expected: No errors

- [ ] **Step 5: Commit**

```bash
git add cmd/run.go cmd/root.go
git commit -m "feat(cli): implement run command and profile dispatch"
```

---

### Task 13: End-to-End Smoke Test

**Files:**
- Modify: `internal/engine/engine_test.go`

- [ ] **Step 1: Build the binary and test manually**

Run:
```bash
go build -o passauto .
./passauto --help
```

Expected: Help output showing all subcommands (init, add, list, remove, run)

- [ ] **Step 2: Run all unit tests**

Run:
```bash
go test ./... -v
```

Expected: All tests PASS

- [ ] **Step 3: Run vet and check for issues**

Run:
```bash
go vet ./...
```

Expected: No issues

- [ ] **Step 4: Commit any fixes**

If any fixes were needed:
```bash
git add -A
git commit -m "fix: address issues found during smoke testing"
```

- [ ] **Step 5: Final commit — tag v0.1.0**

```bash
git tag v0.1.0
```

---
