# Contributing

## PR Commands

Comment these in any PR to trigger workflows:

| Command | Action |
|---------|--------|
| `/rerun` | Re-run all failed CI jobs |
| `/test` | Run tests on all platforms (with race detector) |
| `/benchmark` | Run benchmarks and post results as a comment |

## Development

```bash
# Build
make build

# Test
go test -v ./...

# Lint (requires golangci-lint)
golangci-lint run ./...
```

## CI Checks

All PRs must pass before merging:
- **lint** — golangci-lint
- **security** — gosec + govulncheck
- **test** — ubuntu, macOS, windows
