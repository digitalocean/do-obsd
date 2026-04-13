#!/bin/sh
# vim: noexpandtab

set -ue

SVC_NAME=do-obsd
OTELCOL_SVC_NAME=do-otelcol
OTELCOL_CONFIG_DIR=/etc/${OTELCOL_SVC_NAME}
UPDATER_TIMER=${SVC_NAME}-update.timer

abort_perm() {
	echo "ERROR: $1" >&2
	exit 1
}

# Path exists as a real directory or is created; symlinks and non-directories rejected.
_ensure_real_dir() {
	_path="$1"
	if [ -L "${_path}" ]; then
		abort_perm "Refusing to change permissions on symlink: ${_path}"
	fi
	if [ -e "${_path}" ]; then
		if [ ! -d "${_path}" ]; then
			abort_perm "Expected directory at: ${_path}"
		fi
	else
		mkdir -p "${_path}" || abort_perm "Cannot create directory: ${_path}"
	fi
}

# Path must exist as a regular file from the package; symlinks rejected.
_ensure_regular_file() {
	_path="$1"
	if [ -L "${_path}" ]; then
		abort_perm "Refusing to change permissions on symlink: ${_path}"
	fi
	if [ ! -e "${_path}" ]; then
		abort_perm "Required file missing (expected from package): ${_path}"
	fi
	if [ ! -f "${_path}" ]; then
		abort_perm "Expected regular file at: ${_path}"
	fi
}

_apply_owner_mode() {
	_path="$1"
	_owner="$2"
	_mode="$3"
	chown "${_owner}" "${_path}" || abort_perm "chown failed: ${_path}"
	chmod "${_mode}" "${_path}" || abort_perm "chmod failed: ${_path}"
}

# secure_path kind path [args...]
#   dir  path owner mode   — real directory; mkdir -p if missing; chown + chmod; refuse symlinks
#   dirm path mode         — same as dir for validation/creation, chmod only (owner unchanged)
#   filem path mode        — regular file from package; must exist; chmod only; refuse symlinks
#   file path owner mode   — regular file from package; must exist; chown + chmod; refuse symlinks
secure_path() {
	_kind="$1"
	shift
	case "${_kind}" in
	dir)
		_ensure_real_dir "$1"
		_apply_owner_mode "$1" "$2" "$3"
		;;
	dirm)
		_ensure_real_dir "$1"
		chmod "$2" "$1" || abort_perm "chmod failed: $1"
		;;
	filem)
		_ensure_regular_file "$1"
		chmod "$2" "$1" || abort_perm "chmod failed: $1"
		;;
	file)
		_ensure_regular_file "$1"
		_apply_owner_mode "$1" "$2" "$3"
		;;
	*)
		abort_perm "secure_path: unknown kind: ${_kind}"
		;;
	esac
}

main() {
	create_users
	set_permissions

	systemctl daemon-reload || true

	echo "enable systemd services"
	# Start the config watcher before do-obsd so the initial WriteConfig triggers a reload.
	systemctl enable -f ${OTELCOL_SVC_NAME}-config.path || true
	systemctl start ${OTELCOL_SVC_NAME}-config.path || true

	systemctl enable -f ${SVC_NAME} || true
	systemctl restart ${SVC_NAME} || true

	# do-otelcol.service is managed by systemd directly; do-obsd writes the config
	# and the path unit restarts do-otelcol whenever the config file changes.
	systemctl enable -f ${OTELCOL_SVC_NAME} || true
	systemctl restart ${OTELCOL_SVC_NAME} || true

	patch_updates
}

create_users() {
	useradd -r -s /sbin/nologin -d /opt/digitalocean ${SVC_NAME} 2>/dev/null || true
	useradd -r -s /sbin/nologin -d /opt/digitalocean ${OTELCOL_SVC_NAME} 2>/dev/null || true
	if getent group adm >/dev/null 2>&1; then
		usermod -aG adm "${OTELCOL_SVC_NAME}" || true
	fi
}

set_permissions() {
	# Collector binary is shipped by the package directly to bin/; mode only (owner from package)
	secure_path filem "/opt/digitalocean/bin/${OTELCOL_SVC_NAME}" 755

	# Config: supervisor (do-obsd) writes, collector (do-otelcol) reads.
	# The setgid bit (2750) causes new files created by do-obsd in this directory
	# to inherit the do-otelcol group automatically, so atomic config writes
	# (temp file + rename) are readable by the collector without an explicit chown.
	secure_path dir "${OTELCOL_CONFIG_DIR}" "${SVC_NAME}:${OTELCOL_SVC_NAME}" 2750
	secure_path file "${OTELCOL_CONFIG_DIR}/config.yaml" "${SVC_NAME}:${OTELCOL_SVC_NAME}" 640
}

patch_updates() {
	# systemd timer scheduling.
	echo "enable updater timer"
	systemctl enable -f ${UPDATER_TIMER} || true
	systemctl restart ${UPDATER_TIMER} || true
}

main
