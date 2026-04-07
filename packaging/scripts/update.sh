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

  do_upgrade "${platform}"
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
  rpm) rpm -q ${SVC_NAME} --qf '%{VERSION}' 2>/dev/null ;;
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
    ;;
  rpm)
    CANDIDATE_VER=$(yum -q --disablerepo="*" --enablerepo="${SVC_NAME}" list available ${SVC_NAME} 2>/dev/null \
      | awk '/^do-obsd/{print $2}' | cut -d- -f1)
    # If nothing available, candidate equals local (already latest).
    [ -z "${CANDIDATE_VER}" ] && CANDIDATE_VER="${LOCAL_VER}"
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
    yum -q -y update ${SVC_NAME}
    ;;
  esac
}

# ── Helpers ──────────────────────────────────────────────────────────────────
#To prevent simultaneous modifications. If two processes tried to write package metadata at the same time, you'd corrupt the database.
run_apt_with_retry() {
  _attempt=1
  while [ "${_attempt}" -le "${APT_RETRIES}" ]; do
    _tmp_log=$(mktemp)
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
