#!/bin/bash
set -e

# Only clean up on full purge, not on upgrade or remove.
if [ "$1" = "purge" ]; then
    rm -f /etc/polkit-1/rules.d/60-do-obsd.rules
    rm -rf /etc/do-otelcol
    userdel do-otelcol 2>/dev/null || true
    userdel do-obsd 2>/dev/null || true
    systemctl daemon-reload
fi
