# Contributing to go-update

## Prerequisites

- Go 1.24+
- `golangci-lint` ([install guide](https://golangci-lint.run/welcome/install/))

## Getting started

```bash
git clone https://github.com/bluefunda/go-update
cd go-update
go build ./...
go test -race ./...
```

## Making changes

1. Fork the repo and create a branch: `git checkout -b fix/my-fix`
2. Make your changes
3. Ensure all checks pass:
   ```bash
   go build ./...
   go test -race ./...
   go vet ./...
   golangci-lint run
   ```
4. Commit using [Conventional Commits](https://www.conventionalcommits.org/): `fix:`, `feat:`, `chore:`, etc.
5. Open a pull request against `main`

## Adding a new installation method

1. Add a new `Method` constant in `detector.go`
2. Implement the `Upgrader` interface in `upgrader.go`
3. Wire it in `newUpgrader`
4. Add detection logic in `detectMethod`
5. Add tests in `upgrader_test.go` and `detector_test.go`

## Testing

Tests mock `execCommand` and `httpClient` to avoid real network/system calls:

```go
origExec := execCommand
execCommand = func(name string, args ...string) *exec.Cmd {
    // return a fake command
}
defer func() { execCommand = origExec }()
```

The GitHub asset server is mocked with `httptest.NewServer`.

## License

By contributing you agree that your contributions will be licensed under the Apache 2.0 License.
