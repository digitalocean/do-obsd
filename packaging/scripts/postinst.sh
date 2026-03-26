#!/bin/bash
set -e

# Supervisor runs as its own isolated user
useradd -r -s /sbin/nologin -d /opt/digitalocean do-obsd 2>/dev/null || true

# Collector runs as its own isolated user; needs adm group for kern.log on GPU images
useradd -r -s /sbin/nologin -d /opt/digitalocean do-otelcol 2>/dev/null || true
usermod -aG adm do-otelcol

# do-obsd installs the collector binary to bin/ at startup (OpAMP binary install path).
# The bin dir must be writable by do-obsd.
chown do-obsd:do-obsd /opt/digitalocean/bin
chmod 755 /opt/digitalocean/bin

# Bundle is read-only; do-obsd reads from here, copies to bin/.
chmod 755 /opt/digitalocean/bundle/do-otelcol

# Config: supervisor (do-obsd) writes, collector (do-otelcol) reads
chown do-obsd:do-otelcol /etc/do-otelcol
chmod 750 /etc/do-otelcol
chown do-obsd:do-otelcol /etc/do-otelcol/config.yaml
chmod 640 /etc/do-otelcol/config.yaml

# Polkit: do-obsd user may manage do-otelcol.service only
mkdir -p /etc/polkit-1/rules.d
cat > /etc/polkit-1/rules.d/60-do-obsd.rules <<'EOF'
polkit.addRule(function(action, subject) {
    if (action.id == "org.freedesktop.systemd1.manage-units" &&
        action.lookup("unit") == "do-otelcol.service" &&
        subject.user == "do-obsd") {
        return polkit.Result.YES;
    }
});
EOF

systemctl daemon-reload

# do-obsd manages do-otelcol lifecycle — it installs the binary and starts the service.
# do-otelcol.service is NOT enabled here; do-obsd starts it after binary placement.
systemctl enable do-obsd
systemctl restart do-obsd
