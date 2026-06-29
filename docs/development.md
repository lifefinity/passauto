# Development Guide

## Prerequisites

- Go 1.23+
- golangci-lint (`go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest`)
- gosec (`go install github.com/securego/gosec/v2/cmd/gosec@latest`)
- govulncheck (`go install golang.org/x/vuln/cmd/govulncheck@latest`)

## Getting Started

```bash
cd projects/passauto
go mod download
go build -o passauto.exe .
```

## Makefile Targets

| Target | Description |
|--------|-------------|
| `make deps` | Download and tidy modules |
| `make build` | Compile to `bin/passauto.exe` |
| `make run` | Run via `go run` |
| `make test` | Run all tests with verbose output |
| `make fmt` | Format code with `go fmt` |
| `make vet` | Static analysis with `go vet` |
| `make lint` | Lint with `golangci-lint` |
| `make sec` | Security scan with `gosec` |
| `make vuln` | Vulnerability check with `govulncheck` |
| `make check` | Full pipeline: fmt → vet → lint → sec → vuln → test |
| `make clean` | Remove build artifacts |
| `make install` | Install to `GOPATH/bin` |

## Project Layout

```
passauto/
├── main.go                     # Entry point
├── cmd/                        # CLI commands (cobra)
│   ├── root.go                 # Root command + profile dispatch
│   ├── add.go                  # `passauto add` command
│   ├── update.go               # `passauto update` command
│   ├── list.go                 # `passauto list` command
│   ├── remove.go               # `passauto remove` command
│   ├── run.go                  # runProfile/runProfileWithSteps logic
│   ├── version.go              # `passauto version` command
│   ├── init_cmd.go             # `passauto init` setup
│   └── helpers.go              # Shared utilities (dataPath, deriveKey, loadData)
├── internal/
│   ├── crypto/                 # Encryption (AES-256-GCM, HKDF key derivation)
│   │   ├── crypto.go
│   │   └── crypto_test.go
│   ├── data/                   # Profile storage (JSON load/save/validate)
│   │   ├── data.go
│   │   └── data_test.go
│   ├── engine/                 # PTY execution engine
│   │   ├── engine.go           # Run() entry point, ANSI stripping
│   │   ├── matcher.go          # Regex pattern matching
│   │   ├── stepper.go          # Post-login step execution
│   │   ├── pty_windows.go      # Windows ConPTY implementation
│   │   ├── pty_unix.go         # Linux/macOS PTY implementation
│   │   ├── engine_test.go
│   │   └── matcher_test.go
│   └── testutil/               # Test helpers
│       └── mockprompt.go
├── testdata/                   # Test fixtures (SSH keys)
├── docs/                       # Documentation
├── go.mod
├── go.sum
├── Makefile
└── README.md
```

## Running Tests

```bash
# All tests
make test

# Specific package
go test ./internal/engine/... -v

# With coverage
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out
```

## Adding a New Command

1. Create `cmd/<name>.go`
2. Define a `cobra.Command`
3. Register in `init()` with `rootCmd.AddCommand()`
4. Implement the `RunE` function

## Adding a New Engine Feature

1. Add logic in `internal/engine/`
2. If platform-specific, use build tags (`//go:build windows` / `//go:build !windows`)
3. Wire into `Options` struct in `engine.go`
4. Update `pty_windows.go` and `pty_unix.go` as needed

## Key Design Decisions

- **Cobra** for CLI framework — subcommands, flags, help generation
- **ConPTY / creack/pty** for cross-platform pseudo-terminal support
- **SSH key as encryption seed** — no separate master password to manage
- **Single JSON file** for all profile data — simple, portable, easy to backup
- **Regex patterns** for flexible prompt matching (Go `regexp` syntax)
- **ANSI stripping** before pattern matching to handle colored terminal output

## Windows Development Notes

- ConPTY requires Windows 10 1809+
- Build with `GOOS=windows GOARCH=amd64 go build`
- The `pty_windows.go` file uses `//go:build windows` tag
- Uses `kernel32.dll` directly via `syscall.NewLazyDLL`

## Cross-Compilation

```bash
# Linux
GOOS=linux GOARCH=amd64 go build -o bin/passauto .

# macOS
GOOS=darwin GOARCH=arm64 go build -o bin/passauto .

# Windows
GOOS=windows GOARCH=amd64 go build -o bin/passauto.exe .
```
