#!/bin/sh
# vim: noexpandtab

set -ue

SVC_NAME=do-obsd
OTELCOL_SVC_NAME=do-otelcol
OTELCOL_CONFIG_DIR=/etc/${OTELCOL_SVC_NAME}
POLKIT_RULES=/etc/polkit-1/rules.d/60-${SVC_NAME}.rules
SUDOERS_DROPIN=/etc/sudoers.d/${SVC_NAME}
INSTALL_DIR=/opt/digitalocean/${SVC_NAME}
CRON_SCHEDULE=/etc/cron.hourly
CRON=${CRON_SCHEDULE}/${SVC_NAME}

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
	configure_polkit
	configure_sudoers

	systemctl daemon-reload

	# do-obsd manages do-otelcol lifecycle — it installs the binary and starts the service.
	# do-otelcol.service is NOT enabled here; do-obsd starts it after binary placement.
	echo "enable systemd service"
	systemctl enable -f ${SVC_NAME}
	systemctl restart ${SVC_NAME}

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
	[ -f "${CRON}" ] && rm -f "${CRON}"
	script="${INSTALL_DIR}/scripts/update.sh"
	mkdir -p ${CRON_SCHEDULE}

	cat <<-EOF >"${CRON}"
	#!/bin/sh
	/bin/bash ${script} >/var/log/${SVC_NAME}.update.log 2>&1
	EOF

	chmod +x "${CRON}"
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

# Polkit often still prompts for auth from non-interactive systemd units; sudoers allows
# passwordless /usr/bin/systemctl for do-otelcol only (matches do-obsd binary behavior).
configure_sudoers() {
	cat >"${SUDOERS_DROPIN}" <<'EOF'
# Managed by do-obsd package — allow supervisor to start/stop the collector without TTY auth.
do-obsd ALL=(root) NOPASSWD: /usr/bin/systemctl start do-otelcol.service, /usr/bin/systemctl stop do-otelcol.service, /usr/bin/systemctl restart do-otelcol.service, /usr/bin/systemctl reload do-otelcol.service
EOF
	chmod 0440 "${SUDOERS_DROPIN}" || abort_perm "chmod sudoers drop-in failed"
	if command -v visudo >/dev/null 2>&1; then
		visudo -cf "${SUDOERS_DROPIN}" || abort_perm "sudoers drop-in failed validation"
	fi
}

main
