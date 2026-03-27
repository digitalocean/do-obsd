#!/bin/sh
# vim: noexpandtab

set -ue

SVC_NAME=do-obsd
OTELCOL_SVC_NAME=do-otelcol
OTELCOL_CONFIG_DIR=/etc/${OTELCOL_SVC_NAME}
POLKIT_RULES=/etc/polkit-1/rules.d/60-${SVC_NAME}.rules

main() {
	create_users
	set_permissions
	configure_polkit

	systemctl daemon-reload

	# do-obsd manages do-otelcol lifecycle — it installs the binary and starts the service.
	# do-otelcol.service is NOT enabled here; do-obsd starts it after binary placement.
	echo "enable systemd service"
	systemctl enable -f ${SVC_NAME}
	systemctl restart ${SVC_NAME}
}

create_users() {
	useradd -r -s /sbin/nologin -d /opt/digitalocean ${SVC_NAME} 2>/dev/null || true
	useradd -r -s /sbin/nologin -d /opt/digitalocean ${OTELCOL_SVC_NAME} 2>/dev/null || true
	usermod -aG adm ${OTELCOL_SVC_NAME}
}

set_permissions() {
	# do-obsd installs the collector binary to bin/ at startup
	chown ${SVC_NAME}:${SVC_NAME} /opt/digitalocean/bin
	chmod 755 /opt/digitalocean/bin

	# Bundle is read-only; do-obsd reads from here, copies to bin/
	chmod 755 /opt/digitalocean/bundle/${OTELCOL_SVC_NAME}

	# Config: supervisor (do-obsd) writes, collector (do-otelcol) reads
	chown ${SVC_NAME}:${OTELCOL_SVC_NAME} ${OTELCOL_CONFIG_DIR}
	chmod 750 ${OTELCOL_CONFIG_DIR}
	chown ${SVC_NAME}:${OTELCOL_SVC_NAME} ${OTELCOL_CONFIG_DIR}/config.yaml
	chmod 640 ${OTELCOL_CONFIG_DIR}/config.yaml
}

configure_polkit() {
	mkdir -p /etc/polkit-1/rules.d
	cat > "${POLKIT_RULES}" <<'POLKIT'
polkit.addRule(function(action, subject) {
    if (action.id == "org.freedesktop.systemd1.manage-units" &&
        action.lookup("unit") == "do-otelcol.service" &&
        subject.user == "do-obsd") {
        return polkit.Result.YES;
    }
});
POLKIT
}

main
