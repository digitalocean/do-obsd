# do-obsd

DigitalOcean observability supervisor. Manages the lifecycle of the OpenTelemetry Collector (`do-otelcol`) on Droplets.

## Overview

`do-obsd` is a lightweight Go daemon that:
- Installs the collector binary at startup (OpAMP binary delivery path)
- Starts and stops `do-otelcol.service` via systemd using least-privilege `sudo` (scoped to `do-otelcol.service` only — see `configure_sudoers` in `packaging/scripts/after_install.sh`)
- Runs as an isolated `do-obsd` system user; collector runs as `do-otelcol`

## Compatibility

`do-obsd` currently supports:

- Ubuntu (oldest [End Of Standard Support](https://wiki.ubuntu.com/Releases) LTS release and later)
- Debian ([oldest supported](https://wiki.debian.org/LTS) LTS release and later)
- Fedora 42+
- CentOS Stream 9+
- AlmaLinux 8+
- Rocky Linux 8+

`systemd` is required — the package uses systemd units for service management and systemd timers for automatic updates.

## Development

```bash
# Build
make build

# Test
make test

# Lint
make lint
```
