# do-obsd

DigitalOcean observability supervisor. Manages the lifecycle of the OpenTelemetry Collector (`do-otelcol`) on Droplets.

## Overview

`do-obsd` is a lightweight Go daemon that:
- Installs the collector binary at startup (OpAMP binary delivery path)
- Starts and stops `do-otelcol.service` via systemd
- Runs as an isolated `do-obsd` system user; collector runs as `do-otelcol`

## Why `systemctl` needs `sudo` (and what we changed)

The supervisor runs as **`do-obsd`**, not root. Starting another unit (`do-otelcol.service`) goes through **systemd** and **polkit**. On some images (including many **GPU** Droplets), polkit denies that action for non-interactive service contexts and `systemctl` fails with:

`Failed to start do-otelcol.service: Interactive authentication required.`

A polkit rule alone is not reliable here because rule matching varies by OS/polkit version.

**What we implemented:**

1. **Code** ([`internal/collector/collector.go`](internal/collector/collector.go)) — `Start` and `Stop` invoke  
   `/usr/bin/sudo -n /usr/bin/systemctl start|stop do-otelcol.service`  
   (`-n` = non-interactive: fail if a password would be required.)

2. **Packaging** — [`packaging/scripts/after_install.sh`](packaging/scripts/after_install.sh) (`configure_sudoers`) installs **`/etc/sudoers.d/do-obsd`**: passwordless `sudo` **only** for those `systemctl` commands as user `do-obsd`.  
   [`packaging/scripts/after_remove.sh`](packaging/scripts/after_remove.sh) removes that file on package purge.

3. **Manual setup (before the CDN has a new package)** — Use the commands under **Hotfix** below; they match `configure_sudoers` in `after_install.sh` and replace the supervisor binary. After a new `.deb` is published, `install.sh` / `apt install` applies this without those steps.

**Check that both services are up:**

```bash
systemctl is-active do-obsd do-otelcol
```

## Running on a Droplet (demo)

Typical flow: copy the installer to the Droplet, then run it as root. It configures apt/yum, imports the repo signing key, and installs the **`do-obsd` package from DigitalOcean’s package repo** (same behavior as `curl … | sudo bash` from the CDN).

You do **not** need to build this repo first for that path—the binary comes from the repository, not from your local `make build` output.

```bash
# From your machine: copy the installer (example paths)
scp packaging/scripts/install.sh root@YOUR_DROPLET_IP:/root/

# On the Droplet
sudo bash /root/install.sh
```

`install.sh` checks that the machine is a DigitalOcean Droplet (`check_do`) and only supports `x86_64` Debian/Ubuntu or common RHEL-family distros—see the script for details.

### Trying your own `make build` on a Droplet

The stock `install.sh` does **not** install artifacts from your laptop. To exercise a binary you built locally, install `do-obsd` from the repo first (so users, systemd units, and paths exist), then build for Linux and replace the installed binary, for example:

```bash
# On your machine (Linux binary for amd64 Droplets)
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/do-obsd ./cmd/do-obsd
scp bin/do-obsd root@YOUR_DROPLET_IP:/opt/digitalocean/bin/do-obsd
# On the Droplet
sudo systemctl restart do-obsd
```

### Hotfix before a new package is published (GPU / strict polkit images)

Build a **linux/amd64** supervisor, copy it to the Droplet, then create **`/etc/sudoers.d/do-obsd`** with the same rules as **`configure_sudoers`** in [`after_install.sh`](packaging/scripts/after_install.sh) (if you change the rules in one place, update the other):

```bash
# On your machine
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/do-obsd-linux-amd64 ./cmd/do-obsd
scp bin/do-obsd-linux-amd64 root@YOUR_DROPLET_IP:/root/

# On the Droplet (as root)
tee /etc/sudoers.d/do-obsd >/dev/null <<'EOF'
# Managed by do-obsd package — allow supervisor to start/stop the collector without TTY auth.
do-obsd ALL=(root) NOPASSWD: /usr/bin/systemctl start do-otelcol.service, /usr/bin/systemctl stop do-otelcol.service, /usr/bin/systemctl restart do-otelcol.service, /usr/bin/systemctl reload do-otelcol.service
EOF
chmod 440 /etc/sudoers.d/do-obsd
visudo -cf /etc/sudoers.d/do-obsd
install -m 755 /root/do-obsd-linux-amd64 /opt/digitalocean/bin/do-obsd
systemctl restart do-obsd
```

## Development

```bash
# Build
make build

# Test
make test

# Lint
make lint
```
