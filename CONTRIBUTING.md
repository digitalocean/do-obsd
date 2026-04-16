# Contributing to do-obsd

Thanks for your interest in this project.

All contributors and contributions agree to abide by the [Code of Conduct](CODE_OF_CONDUCT.md).

## How to propose changes

1. Open an issue to discuss substantial changes when it helps align on design or scope.
2. Fork the repository and create a focused branch for your work.
3. Run the checks below locally before opening a pull request.
4. Open a pull request with a clear description of the problem, the change, and any trade-offs.

## Development

Requirements:

- Go version matching `go.mod` (see the `go` directive).
- Docker (optional but used by `make lint` for `golangci-lint` and `shellcheck`).

```bash
make build   # build ./cmd/do-obsd
make test    # go test -race -cover ./...
make lint    # shell scripts + golangci-lint (via Docker)
```

Packaging and install logic live under `packaging/`; keep shell changes compatible with `sh` where the scripts use `#!/bin/sh`.

## Security

Do not open public issues for security vulnerabilities. Follow [`SECURITY.md`](SECURITY.md).

## License

By contributing, you agree that your contributions will be licensed under the same terms as the project (Apache License 2.0 — see [`LICENSE`](LICENSE)).
