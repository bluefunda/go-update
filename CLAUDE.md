# CLAUDE.md — go-update

## What is this?

Shared Go library that provides CLI self-update functionality for BlueFunda CLIs (`bai`, `abaper`, `breq`).
Detects the installation method (Homebrew, dpkg, rpm, standalone binary) and upgrades using the
appropriate strategy.

Module: `github.com/bluefunda/go-update` | Go 1.24+ | License: Apache 2.0

## Build & Verify

```bash
go build ./...
go test -race ./...
go vet ./...
golangci-lint run
```

All four must pass before any change is considered complete.

## Architecture

| File | Purpose |
|------|---------|
| `update.go` | `Run(Config)` — top-level entry point: check → diff → prompt → upgrade |
| `checker.go` | `fetchLatest` — fetches latest release tag from GitHub Releases API |
| `detector.go` | `detectMethod` — infers install method from the binary's resolved path |
| `upgrader.go` | `Upgrader` interface + four implementations: Brew, Dpkg, Rpm, Binary |

### Installation methods

| Method | Detection | Strategy |
|--------|-----------|----------|
| `MethodBrew` | Path contains `homebrew`, `linuxbrew`, or `/cellar/` | `brew update --quiet` then `brew upgrade <name>`, verify version |
| `MethodDpkg` | `dpkg -S <exe>` succeeds | Download `.deb` from GitHub Releases, `sudo dpkg -i` |
| `MethodRpm` | `rpm -qf <exe>` succeeds | Download `.rpm` from GitHub Releases, `sudo rpm -U` |
| `MethodBinary` | Fallback | Download archive from GitHub Releases, verify SHA-256, replace in-place |

## Conventions

- Commits: conventional format (`feat:`, `fix:`, `chore:`)
- Branches: `<type>/<short-description>`
- PRs: squash-merged to `main`
- Releases: release-please + semver tags

## Consuming this library

```go
import "github.com/bluefunda/go-update"

var updateCmd = &cobra.Command{
    Use:   "update",
    Short: "Update <cli> to the latest version",
    RunE: func(cmd *cobra.Command, args []string) error {
        return update.Run(update.Config{
            BinaryName:     "mycli",
            CurrentVersion: Version, // injected via ldflags at build time
            GitHubOwner:    "myorg",
            GitHubRepo:     "myrepo",
            HomebrewCask:   "mycli",
        })
    },
}
```

## Test patterns

- `execCommand` and `httpClient` vars are overridable for test mocking (see `upgrader_test.go`)
- Tests use `httptest.NewServer` for asset downloads; no real network calls
- `go test -race ./...` is required — the library is safe for concurrent use
