#!/bin/bash
# vim: noexpandtab
#
# Auto-update do-obsd via the native package manager (apt / yum).
# Intended to be invoked by systemd timer; uses flock to prevent overlapping runs.

set -ue
#file used for process locking so only one updater runs at a time
LOCK_FILE="/var/lock/do-obsd-update.lock"
SVC_NAME="do-obsd"
LOCAL_VER=""
CANDIDATE_VER=""
RPM_UPDATE_AVAILABLE="false"
APT_RETRIES="5"
APT_RETRY_DELAY="15"

main() {
  if command -v apt-get >/dev/null 2>&1; then
    platform="deb"
  elif command -v yum >/dev/null 2>&1; then
    platform="rpm"
  else
    not_supported
  fi

  refresh_index "${platform}"
  resolve_versions "${platform}"

  echo "Local version : ${LOCAL_VER}"
  echo "Candidate     : ${CANDIDATE_VER}"

  if [ "${LOCAL_VER}" = "${CANDIDATE_VER}" ]; then
    echo "Already up-to-date"
    exit 0
  fi

  # Only upgrade if candidate version is strictly newer (not equal or older)
  if ! is_candidate_newer "${platform}"; then
    echo "Local version is newer than or equal to candidate; skipping downgrade"
    exit 0
  fi

  do_upgrade "${platform}" || abort "Package upgrade failed"
  echo "Upgrade complete — now at $(resolve_local_ver "${platform}")"
}

# updates package metadata before version comparison/upgrades
refresh_index() {
  platform=${1:-}
  echo "Refreshing package index..."
  case "${platform}" in
  deb)
    export DEBIAN_FRONTEND="noninteractive"
    run_apt_with_retry apt-get -qq update \
      --allow-releaseinfo-change-suite \
      --allow-releaseinfo-change-codename \
      -o Dir::Etc::SourceParts=/dev/null \
      -o APT::Get::List-Cleanup=no \
      -o Dir::Etc::SourceList="sources.list.d/${SVC_NAME}.list"
    ;;
  rpm)
    yum -q -y --disablerepo="*" --enablerepo="${SVC_NAME}" makecache
    ;;
  esac
}

# ── Resolve installed + candidate versions from package manager ──────────────
resolve_local_ver() {
  platform=${1:-}
  case "${platform}" in
  deb) dpkg -s ${SVC_NAME} 2>/dev/null | awk '/^Version:/{print $2}' ;;
  rpm) rpm -q ${SVC_NAME} --qf '%{EPOCHNUM}:%{VERSION}-%{RELEASE}' 2>/dev/null ;;
  esac
}

resolve_versions() {
  platform=${1:-}
  LOCAL_VER=$(resolve_local_ver "${platform}")
  if [ -z "${LOCAL_VER}" ]; then
    abort "Cannot determine installed version of ${SVC_NAME}"
  fi

  case "${platform}" in
  deb)
    CANDIDATE_VER=$(apt-cache policy ${SVC_NAME} | awk '/Candidate:/{print $2}')
    # apt-cache prints "(none)" when no candidate exists in configured repos.
    if [ "${CANDIDATE_VER}" = "(none)" ]; then
      abort "No candidate version available for ${SVC_NAME} (apt reports Candidate: (none))"
    fi
    ;;
  rpm)
    # yum check-update returns:
    #   0   => no updates
    #   100 => updates available
    #   else => error (repo/network/config), must not be treated as "no update"
    _yum_output=$(yum -q --disablerepo="*" --enablerepo="${SVC_NAME}" check-update ${SVC_NAME} 2>/dev/null)
    _yum_rc=$?
    case "${_yum_rc}" in
    0)
      RPM_UPDATE_AVAILABLE="false"
      CANDIDATE_VER="${LOCAL_VER}"
      ;;
    100)
      RPM_UPDATE_AVAILABLE="true"
      # Standard yum output is: package.arch  version  repo.
      CANDIDATE_VER=$(echo "${_yum_output}" | awk -v svc="${SVC_NAME}" '($1 == svc || index($1, svc ".") == 1) {v=$2; if(v !~ /^[0-9]+:/) v="0:"v; print v; exit}')
      [ -z "${CANDIDATE_VER}" ] && abort "yum reported updates but no candidate version was parsed for ${SVC_NAME}"
      ;;
    *)
      abort "yum check-update failed for ${SVC_NAME} (exit ${_yum_rc})"
      ;;
    esac
    ;;
  esac

  if [ -z "${CANDIDATE_VER}" ]; then
    abort "Cannot determine candidate version of ${SVC_NAME}"
  fi
}

# ── Perform upgrade ──────────────────────────────────────────────────────────
do_upgrade() {
  platform=${1:-}
  echo "Upgrading ${SVC_NAME} ${LOCAL_VER} -> ${CANDIDATE_VER}"
  case "${platform}" in
  deb)
    run_apt_with_retry apt-get \
      -o Dpkg::Options::="--force-confdef" \
      -o Dpkg::Options::="--force-confold" \
      -qq install -y --only-upgrade ${SVC_NAME}
    ;;
  rpm)
    yum -q -y update ${SVC_NAME} || return 1
    ;;
  esac
}

# ── Version comparison: only upgrade if candidate is strictly newer ─────────
is_candidate_newer() {
  platform=${1:-}
  case "${platform}" in
  deb)
    # dpkg --compare-versions: returns 0 if ver1 op ver2 is true
    # Only upgrade if candidate is strictly newer than installed
    dpkg --compare-versions "${LOCAL_VER}" lt "${CANDIDATE_VER}"
    ;;
  rpm)
    # For RPM, prefer rpmdev-vercmp (standard tool for EVR comparison)
    # If unavailable, rely on yum check-update's update signal captured in resolve_versions().
    if command -v rpmdev-vercmp >/dev/null 2>&1; then
      # rpmdev-vercmp outputs the relationship (e.g., "ver1 < ver2")
      local output
      output=$(rpmdev-vercmp "${LOCAL_VER}" "${CANDIDATE_VER}" 2>/dev/null)
      if echo "${output}" | grep -q '<'; then
        return 0  # candidate is newer, upgrade needed
      fi
      return 1   # candidate is not newer
    else
      # yum check-update returns 100 only when a newer package is available.
      # This avoids unsafe inequality checks that can misclassify downgrades.
      [ "${RPM_UPDATE_AVAILABLE}" = "true" ]
    fi
    ;;
  esac
}

# ── Helpers ──────────────────────────────────────────────────────────────────
#To prevent simultaneous modifications. If two processes tried to write package metadata at the same time, you'd corrupt the database.
run_apt_with_retry() {
  _attempt=1
  while [ "${_attempt}" -le "${APT_RETRIES}" ]; do
    _tmp_log=$(mktemp) || {
      echo "ERROR: Failed to create temporary log file" >&2
      return 1
    }
    
    if "$@" >"${_tmp_log}" 2>&1; then
      cat "${_tmp_log}"
      rm -f "${_tmp_log}"
      return 0
    fi

    cat "${_tmp_log}" >&2
    if grep -Eq 'Could not get lock|Unable to lock directory' "${_tmp_log}"; then
      if [ "${_attempt}" -lt "${APT_RETRIES}" ]; then
        echo "APT is locked by another process, retrying in ${APT_RETRY_DELAY}s (attempt ${_attempt}/${APT_RETRIES})"
        rm -f "${_tmp_log}"
        sleep "${APT_RETRY_DELAY}"
        _attempt=$((_attempt + 1))
        continue
      fi
    fi

    rm -f "${_tmp_log}"
    return 1
  done
}

not_supported() {
  cat <<-EOF

	This script does not support the OS/Distribution on this machine.
	If you feel that this is an error contact support@digitalocean.com

	EOF
  exit 2
}

abort() {
  echo "ERROR: $1" >/dev/stderr
  exit 1
}

# ── Entry point: flock prevents overlapping runs ─────────────────────────────
exec 200>"${LOCK_FILE}"
if ! flock -n 200; then
  echo "Another update is already running, exiting."
  exit 0
fi
main
