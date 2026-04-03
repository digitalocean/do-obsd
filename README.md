# do-obsd

DigitalOcean observability supervisor. Manages the lifecycle of the OpenTelemetry Collector (`do-otelcol`) on Droplets.

## Overview

`do-obsd` is a lightweight Go daemon that:
- Installs the collector binary at startup (OpAMP binary delivery path)
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

## CI and releases

Pull requests and branch pushes run **test** and **lint** via GitHub Actions.

Pushing a **semver tag** `MAJOR.MINOR.PATCH` (e.g. `1.2.3`) runs **cthulhu-release-dispatch**, which notifies the **Cthulhu** monorepo (`repository_dispatch` event `do-obsd-release`) to build and publish packages. The repo must define the Actions secret **`CTHULHU_DISPATCH_TOKEN`** (token able to create that dispatch on `digitalocean/cthulhu` on internal GitHub Enterprise).

You can also re-trigger dispatch from the Actions tab: **cthulhu-release-dispatch** → **Run workflow** → enter the tag version.
