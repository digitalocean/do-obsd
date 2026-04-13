#!/bin/sh
# vim: noexpandtab

set -ue

SVC_NAME=do-obsd
OTELCOL_SVC_NAME=do-otelcol
OTELCOL_CONFIG_DIR=/etc/${OTELCOL_SVC_NAME}
CRON_SCHEDULE=/etc/cron.hourly
CRON=${CRON_SCHEDULE}/${SVC_NAME}
UPDATER_SVC=${SVC_NAME}-update.service
UPDATER_TIMER=${SVC_NAME}-update.timer

# fix an issue where this script runs on upgrades for rpm
# see https://github.com/jordansissel/fpm/issues/1175#issuecomment-240086016
arg="${1:-0}"

main() {
	if [ "${arg}" -gt 0 ] 2>/dev/null; then
		# rpm upgrade
		exit 0
	elif [ "${arg}" = "upgrade" ]; then
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
	rm -fv "/etc/cron.d/${SVC_NAME}" "/etc/cron.hourly/${SVC_NAME}" "/etc/cron.daily/${SVC_NAME}" || true
}

clean_systemd() {
	echo "Cleaning up systemd services"
	systemctl stop ${SVC_NAME}.service || true
	systemctl disable -f ${SVC_NAME}.service || true
	systemctl stop ${UPDATER_TIMER} || true
	systemctl disable -f ${UPDATER_TIMER} || true
	systemctl stop ${UPDATER_SVC} || true
	systemctl disable -f ${UPDATER_SVC} || true
	systemctl stop ${OTELCOL_SVC_NAME}.service || true
	systemctl disable -f ${OTELCOL_SVC_NAME}.service || true
	systemctl daemon-reload || true
}

clean_resources() {
	echo "Removing users and configuration"
	rm -rf "${OTELCOL_CONFIG_DIR}"
	userdel ${OTELCOL_SVC_NAME} 2>/dev/null || true
	userdel ${SVC_NAME} 2>/dev/null || true
}

main
