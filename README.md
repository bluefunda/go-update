# go-update

[![Go Reference](https://pkg.go.dev/badge/github.com/bluefunda/go-update.svg)](https://pkg.go.dev/github.com/bluefunda/go-update)
[![CI](https://github.com/bluefunda/go-update/actions/workflows/ci.yml/badge.svg)](https://github.com/bluefunda/go-update/actions/workflows/ci.yml)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)

Shared Go library for CLI self-update functionality. Detects the installation method and upgrades using the appropriate strategy — no configuration required from users.

## Features

- **Auto-detects** Homebrew, dpkg, rpm, or standalone binary installation
- **Homebrew**: runs `brew update` first so the tap is current, then upgrades and verifies
- **dpkg / rpm**: downloads the package from GitHub Releases and installs with `sudo`
- **Binary**: downloads the archive, verifies SHA-256 checksum, replaces the binary in-place
- **Safe**: never reports success unless the installed version actually changed

## Installation

```bash
go get github.com/bluefunda/go-update
```

## Usage

```go
package cmd

import (
    "github.com/bluefunda/go-update"
    "github.com/spf13/cobra"
)

var updateCmd = &cobra.Command{
    Use:   "update",
    Short: "Update mycli to the latest version",
    Long: `Check for a newer release and upgrade automatically.
The installation method (Homebrew, dpkg, rpm, standalone binary) is
detected from the current executable path.`,
    RunE: func(cmd *cobra.Command, args []string) error {
        return update.Run(update.Config{
            BinaryName:     "mycli",
            CurrentVersion: Version, // injected at build time via ldflags
            GitHubOwner:    "myorg",
            GitHubRepo:     "myrepo",
            HomebrewCask:   "mycli", // formula name in your homebrew tap
        })
    },
}
```

### User experience

```
$ mycli update
Checking for updates...

Current: mycli v1.2.0
Latest:  v1.3.0

Update available.

Continue? [y/N] y

Updating via Homebrew...

==> Upgrading mycli
...
✓ Successfully updated to v1.3.0
```

## How detection works

| Condition | Method |
|-----------|--------|
| Executable path contains `homebrew`, `linuxbrew`, or `/cellar/` | Homebrew |
| `dpkg -S <exe>` succeeds (Linux) | dpkg |
| `rpm -qf <exe>` succeeds (Linux) | rpm |
| None of the above | Binary (direct GitHub Releases download) |

## GitHub Releases format

For the binary upgrader, releases must follow [GoReleaser](https://goreleaser.com) naming conventions:

```
<binary>_<version>_<os>_<arch>.tar.gz   # Linux
<binary>_<version>_<os>_<arch>.zip      # macOS / Windows
checksums.txt                            # SHA-256 checksums
```

dpkg and rpm packages:
```
<binary>_<version>_linux_<arch>.deb
<binary>_<version>_linux_<arch>.rpm
```

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md).

## License

Apache 2.0 — see [LICENSE](LICENSE).
