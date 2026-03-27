#!/bin/sh
# vim: noexpandtab

set -ue

SVC_NAME=do-obsd
OTELCOL_SVC_NAME=do-otelcol
OTELCOL_CONFIG_DIR=/etc/${OTELCOL_SVC_NAME}
POLKIT_RULES=/etc/polkit-1/rules.d/60-${SVC_NAME}.rules
CRON_SCHEDULE=/etc/cron.hourly
CRON=${CRON_SCHEDULE}/${SVC_NAME}

# fix an issue where this script runs on upgrades for rpm
# see https://github.com/jordansissel/fpm/issues/1175#issuecomment-240086016
arg="${1:-0}"

main() {
	if echo "${arg}" | grep -qP '^\d+$' && [ "${arg}" -gt 0 ]; then
		# rpm upgrade
		exit 0
	elif echo "${arg}" | grep -qP '^upgrade$'; then
		# deb upgrade
		exit 0
	fi

	clean_systemd
	remove_cron

	# full cleanup on purge (deb) or complete removal (rpm arg=0)
	if [ "${arg}" = "purge" ] || [ "${arg}" = "0" ]; then
		clean_resources
	fi
}

remove_cron() {
	rm -fv "${CRON}"
}

clean_systemd() {
	echo "Cleaning up systemd services"
	systemctl stop ${SVC_NAME} || true
	systemctl disable ${SVC_NAME}.service || true
	systemctl stop ${OTELCOL_SVC_NAME} || true
	systemctl disable ${OTELCOL_SVC_NAME}.service || true
	systemctl daemon-reload || true
}

clean_resources() {
	echo "Removing users and configuration"
	rm -f "${POLKIT_RULES}"
	rm -rf "${OTELCOL_CONFIG_DIR}"
	userdel ${OTELCOL_SVC_NAME} 2>/dev/null || true
	userdel ${SVC_NAME} 2>/dev/null || true
}

main
