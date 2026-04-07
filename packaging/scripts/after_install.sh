#!/bin/sh
# vim: noexpandtab

set -ue

SVC_NAME=do-obsd
OTELCOL_SVC_NAME=do-otelcol
OTELCOL_CONFIG_DIR=/etc/${OTELCOL_SVC_NAME}
SUDOERS_DROPIN=/etc/sudoers.d/${SVC_NAME}
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
	configure_sudoers

	systemctl daemon-reload || true

	# do-obsd manages do-otelcol lifecycle — it installs the binary and starts the service.
	# do-otelcol.service is NOT enabled here; do-obsd starts it after binary placement.
	echo "enable systemd service"
	systemctl enable -f ${SVC_NAME} || true
	systemctl restart ${SVC_NAME} || true

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
	# do-obsd installs the collector binary to bin/ at startup
	secure_path dir /opt/digitalocean/bin "${SVC_NAME}:${SVC_NAME}" 755

	# Bundle binary is read-only; mode only (owner from package)
	secure_path filem "/opt/digitalocean/bundle/${OTELCOL_SVC_NAME}" 755

	# Config: supervisor (do-obsd) writes, collector (do-otelcol) reads
	secure_path dir "${OTELCOL_CONFIG_DIR}" "${SVC_NAME}:${OTELCOL_SVC_NAME}" 750
	secure_path file "${OTELCOL_CONFIG_DIR}/config.yaml" "${SVC_NAME}:${OTELCOL_SVC_NAME}" 640
}

patch_updates() {
	# systemd timer scheduling.
	echo "enable updater timer"
	systemctl enable -f ${UPDATER_TIMER} || true
	systemctl restart ${UPDATER_TIMER} || true
}

# configure_sudoers grants the do-obsd service user the ability to start, stop, and restart
# do-otelcol.service via systemctl without a password. This is required because do-obsd runs
# as an unprivileged system user and must manage the collector lifecycle from a non-interactive
# context (no TTY), where polkit would otherwise deny the request.
#
# The rule is written to a temp file and validated with visudo -c before being moved into place.
# This prevents a syntax error from breaking all sudo access on the system before the file lands.
configure_sudoers() {
	_tmp=$(mktemp) || abort_perm "cannot create temp file for sudoers drop-in"
	cat >"${_tmp}" <<'EOF'
# Managed by do-obsd package — do not edit manually.
# Grants the do-obsd supervisor passwordless systemctl access to do-otelcol.service only.
do-obsd ALL=(root) NOPASSWD: /usr/bin/systemctl start do-otelcol.service, /usr/bin/systemctl stop do-otelcol.service, /usr/bin/systemctl restart do-otelcol.service
EOF
	# 0440: sudoers files must not be world-writable; some sudo versions refuse to load them otherwise.
	chmod 0440 "${_tmp}" || { rm -f "${_tmp}"; abort_perm "chmod sudoers drop-in failed"; }
	# Validate before moving into place — a bad sudoers file breaks all sudo access system-wide.
	if command -v visudo >/dev/null 2>&1; then
		visudo -cf "${_tmp}" || { rm -f "${_tmp}"; abort_perm "sudoers drop-in failed validation"; }
	fi
	# mv is atomic (rename syscall) — sudo never sees a partially written file.
	mv "${_tmp}" "${SUDOERS_DROPIN}" || { rm -f "${_tmp}"; abort_perm "installing sudoers drop-in failed"; }
}

main
