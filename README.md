# do-obsd

DigitalOcean observability supervisor. Manages the lifecycle of the OpenTelemetry Collector (`do-otelcol`) on Droplets.

## Overview

`do-obsd` is a lightweight Go daemon that:
- Downloads the collector binary from a fixed HTTPS URL at startup
- Starts and stops `do-otelcol.service` via systemd
- Runs as an isolated `do-obsd` system user; collector runs as `do-otelcol`

## Development

```bash
# Build
make build

# Test
make test

# Lint
make lint
```
