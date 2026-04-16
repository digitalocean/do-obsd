# do-obsd

[![Go Report Card](https://goreportcard.com/badge/github.com/digitalocean/do-obsd)](https://goreportcard.com/report/github.com/digitalocean/do-obsd)

DigitalOcean observability supervisor. Manages collector configuration and works with systemd so the OpenTelemetry Collector (`do-otelcol`) runs on supported Linux hosts.

## Overview

`do-obsd` is a lightweight Go daemon that:

- Writes collector configuration (for example under `/etc/do-otelcol/`) as the `do-obsd` system user.
- Relies on systemd to run `do-otelcol`; a path unit watches the config file and restarts or reloads the collector when it changes (see `packaging/scripts/after_install.sh` and units under `packaging/syscfg/systemd/`).
- Runs as an isolated `do-obsd` user; the collector runs as `do-otelcol`.

## Installation

Published packages are installed via the installer script (run as root). Example:

```bash
curl -sSL https://obsd.sfo3.cdn.digitaloceanspaces.com/install.sh | sudo bash
```

The script configures apt or yum for the DigitalOcean package repository. Supported distributions are defined in `packaging/scripts/install.sh` (for example Debian, Ubuntu, and several RPM-based families). The apt path is **amd64** only today.

## Development

```bash
make build
make test
make lint
```

## Project policies

- Security reporting: [`SECURITY.md`](SECURITY.md)
- Contributing: [`CONTRIBUTING.md`](CONTRIBUTING.md)
- Code of Conduct: [`CODE_OF_CONDUCT.md`](CODE_OF_CONDUCT.md)
- License: [`LICENSE`](LICENSE)
- Open source readiness checklist: [`docs/open-source-readiness.md`](docs/open-source-readiness.md)

## Report an issue

Open a [new issue](https://github.com/digitalocean/do-obsd/issues/new/choose) if one does not [already exist](https://github.com/digitalocean/do-obsd/issues). For **security vulnerabilities**, use [`SECURITY.md`](SECURITY.md) instead of a public issue.
