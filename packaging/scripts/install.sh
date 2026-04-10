#!/bin/sh
#   curl -sSL https://obsd.sfo3.cdn.digitaloceanspaces.com/install.sh | sudo bash
#   wget -qO- https://obsd.sfo3.cdn.digitaloceanspaces.com/install.sh | sudo bash

set -u

REPO_DOMAIN="obsd.sfo3.cdn.digitaloceanspaces.com"
REPO_HOST="https://${REPO_DOMAIN}"
REPO_GPG_KEY=${REPO_HOST}/gpg.key
INSTALL_SCRIPT_URL="${REPO_HOST}/install.sh"

branch="do-obsd-preview"

RETRY_CRON_SCHEDULE=/etc/cron.hourly
RETRY_CRON=${RETRY_CRON_SCHEDULE}/do-obsd-install

dist="unknown"
exit_status=0
trap_status=0
no_retry="false"
repo_name=do-obsd
deb_list=/etc/apt/sources.list.d/${repo_name}.list
deb_pref=/etc/apt/preferences.d/${repo_name}.pref
deb_keyfile=/usr/share/keyrings/${repo_name}-keyring.gpg
rpm_repo=/etc/yum.repos.d/${repo_name}.repo
ARCH_UNSUPPORTED_EXIT=42

main() {
  [ "$(id -u)" != "0" ] &&
    abort "This script must be executed as root."

  trap 'trap_status=$?; [ "${exit_status}" -eq 0 ] && exit_status=${trap_status}; script_cleanup; exit ${exit_status}' EXIT
  trap 'no_retry="true"; exit_status=130; exit 130' INT TERM

  check_dist

  case "${dist}" in
  debian | ubuntu)
    i=1
    until [ "$i" -ge 6 ]; do
      echo "Installing do-obsd, attempt ${i}"
      install_apt
      exit_status=$?
      if [ ${exit_status} -eq ${ARCH_UNSUPPORTED_EXIT} ]; then
        no_retry="true"
        break
      fi
      if [ ${exit_status} -eq 0 ]; then
        break
      fi
      i=$((i+1))
      sleep 60
    done
    ;;
  centos | fedora | rocky | almalinux)
    i=1
    until [ "$i" -ge 6 ]; do
      echo "Installing do-obsd, attempt ${i}"
      install_yum
      exit_status=$?
      if [ ${exit_status} -eq 0 ]; then
        break
      fi
      i=$((i+1))
      sleep 60
    done
    ;;
  *)
    not_supported
    ;;
  esac

  if [ ${exit_status} -eq 0 ]; then
    ensure_do_agent || true
  fi

  return ${exit_status}
}

patch_retry_install() {
  [ -f "${RETRY_CRON}" ] && rm -f "${RETRY_CRON}"
  mkdir -p ${RETRY_CRON_SCHEDULE}
  if ! command -v crontab >/dev/null 2>&1; then
    echo "cron not installed, installing"
    if command -v apt-get >/dev/null 2>&1; then
      apt-get -qq install -y cron
    elif command -v yum >/dev/null 2>&1; then
      yum install -y cronie
    else
      echo "not supported os"
      return 1
    fi
  fi

  cat <<EOF >"${RETRY_CRON}"
#!/bin/sh
tmp_file=\$(mktemp -t do_obsd.install.XXXXXX)
trap "rm -f \"\${tmp_file}\"" EXIT
url="${INSTALL_SCRIPT_URL}"
log_file="/var/log/do-obsd.install.log"

if command -v curl >/dev/null 2>&1; then
  if ! curl -sSL "\${url}" -o "\${tmp_file}"; then
    now=\$(date +"%T")
    echo "Retry at: \${now} - failed to download install script with curl" >> "\${log_file}"
    exit 1
  fi
elif command -v wget >/dev/null 2>&1; then
  if ! wget -qO "\${tmp_file}" "\${url}"; then
    now=\$(date +"%T")
    echo "Retry at: \${now} - failed to download install script with wget" >> "\${log_file}"
    exit 1
  fi
else
  now=\$(date +"%T")
  echo "Retry at: \${now} - neither curl nor wget is installed; cannot download install script" >> "\${log_file}"
  exit 1
fi

now=\$(date +"%T")
echo "Retry at: \${now}" >> "\${log_file}"
/bin/sh "\${tmp_file}" >> "\${log_file}" 2>&1
EOF

  chmod +x "${RETRY_CRON}"
}

remove_retry_install() {
  rm -f "${RETRY_CRON}"
}

script_cleanup() {
  if [ ${exit_status} -ne 0 ]; then
    if [ ${no_retry} = "false" ]; then
      echo "Install failed, will retry again later"
      patch_retry_install
    else
      echo "Install failed, and will not be retried"
    fi
  else
    echo "DigitalOcean Observability Supervisor is successfully installed"
    remove_retry_install || true
  fi
}

install_deps() {
  platform=${1:-}
  [ -z "${platform}" ] && abort "Destination repository is required. Usage: install_deps <platform>"
  echo "Checking dependencies for installing do-obsd"
  case "${platform}" in
  rpm)
    yum install -y gpgme ca-certificates
    ;;
  deb)
    if ! command -v gpg >/dev/null 2>&1; then
      echo "Installing GNUPG"
      apt-get -qq update || true
      apt-get install -y gnupg2
    fi
    if ! apt-get -qq install -y ca-certificates apt-utils apt-transport-https; then
      apt-get -qq update
      apt-get -qq install -y ca-certificates apt-utils apt-transport-https
    fi
    ;;
  esac
}

install_apt() (
  set -e
  export DEBIAN_FRONTEND=noninteractive

  # Verify architecture before writing repo config; abort without retry on mismatch
  echo "Checking architecture support..."
  _arch=$(dpkg --print-architecture 2>/dev/null || true)
  if [ "${_arch}" != "amd64" ]; then
    echo "ERROR: do-obsd apt repository is amd64-only; detected architecture: ${_arch:-unknown}" >&2
    exit ${ARCH_UNSUPPORTED_EXIT}
  fi

  echo "Setting up do-obsd apt repository..."
  install_deps "deb"

  echo "Importing GPG public key"
  wget -qO- "${REPO_GPG_KEY}" | gpg --dearmor >"${deb_keyfile}"
  # arch=amd64: repo is amd64-only; avoids apt multi-arch confusion on some images.
  echo "deb [signed-by=${deb_keyfile} arch=amd64] ${REPO_HOST}/apt/${branch} main main" >"${deb_list}"
  # Pin by Release Origin/Label/Codename (not the repo hostname — "origin" in apt prefs is
  # the Origin: field from InRelease, which we publish as DigitalOcean / do-obsd).
  cat <<-EOF >${deb_pref}
	Package: *
	Pin: release o=DigitalOcean,l=do-obsd,n=main
	Pin-Priority: 500
	EOF

  echo "Installing do-obsd"
  apt-get -q update
  apt-get -q --fix-missing install -y do-obsd
)

install_yum() (
  set -e

  echo "Setting up do-obsd yum repository..."
  install_deps "rpm"

  cat <<-EOF >${rpm_repo}
	[${repo_name}]
	name=DigitalOcean Observability Supervisor
	baseurl=${REPO_HOST}/yum/${branch}/\$basearch
	repo_gpgcheck=1
	gpgcheck=1
	enabled=1
	gpgkey=${REPO_GPG_KEY}
	sslverify=1
	sslcacert=/etc/pki/tls/certs/ca-bundle.crt
	metadata_expire=300
	EOF

  yum --disablerepo="*" --enablerepo="${repo_name}" makecache
  yum install -y do-obsd
)

check_dist() {
  echo "Verifying compatibility with script..."
  if [ -f /etc/os-release ]; then
    dist=$(awk -F= '$1 == "ID" {gsub("\"", ""); print$2}' /etc/os-release)
  elif [ -f /etc/redhat-release ]; then
    dist=$(awk '{print tolower($1)}' /etc/redhat-release)
  else
    not_supported
  fi

  dist=$(echo "${dist}" | tr '[:upper:]' '[:lower:]')

  case "${dist}" in
  debian | ubuntu | centos | fedora | rocky | almalinux)
    echo "OK"
    ;;
  *)
    not_supported
    ;;
  esac
}

not_supported() {
  no_retry="true"
  exit_status=1
  cat <<-EOF

	This script does not support the OS/Distribution on this machine.
	If you feel that this is an error contact support@digitalocean.com

	EOF
  exit ${exit_status}
}

abort() {
  echo "ERROR: $1" >/dev/stderr
  exit 1
}

DO_AGENT_INSTALL_URL="https://repos.insights.digitalocean.com/install.sh"
# Best-effort download only: cap wait so DNS/network stalls cannot hang the main installer.
DO_AGENT_DOWNLOAD_CONNECT_TIMEOUT=20
DO_AGENT_DOWNLOAD_MAX_TIME=120
DO_AGENT_DOWNLOAD_RETRIES=3
DO_AGENT_DOWNLOAD_WGET_TIMEOUT=30
DO_AGENT_DOWNLOAD_WGET_TRIES=3

ensure_do_agent() {
  echo "Checking for do-agent..."

  case "${dist}" in
  debian | ubuntu)
    if dpkg -s do-agent 2>/dev/null | grep -q '^Status: install ok installed'; then
      echo "do-agent is already installed, skipping"
      return 0
    fi
    ;;
  centos | fedora | rocky | almalinux)
    if rpm -q do-agent >/dev/null 2>&1; then
      echo "do-agent is already installed, skipping"
      return 0
    fi
    ;;
  *)
    echo "WARN: unknown distribution '${dist}', skipping do-agent install" >&2
    return 0
    ;;
  esac

  echo "Installing do-agent..."
  # Subshell so EXIT trap runs when this block ends (POSIX sh does not run EXIT on function return).
  (
    tmp_file=$(mktemp -t do_agent_install.XXXXXX) || {
      echo "WARN: could not create temp file for do-agent install" >&2
      exit 1
    }
    if [ -z "${tmp_file}" ]; then
      echo "WARN: mktemp returned empty path, skipping do-agent install" >&2
      exit 1
    fi
    trap 'rm -f "${tmp_file}"' EXIT INT HUP TERM

    if command -v curl >/dev/null 2>&1; then
      if ! curl -fsSL \
        --connect-timeout "${DO_AGENT_DOWNLOAD_CONNECT_TIMEOUT}" \
        --max-time "${DO_AGENT_DOWNLOAD_MAX_TIME}" \
        --retry "${DO_AGENT_DOWNLOAD_RETRIES}" \
        --retry-delay 2 \
        "${DO_AGENT_INSTALL_URL}" -o "${tmp_file}"; then
        echo "WARN: failed to download do-agent install script, continuing without it" >&2
        exit 1
      fi
    elif command -v wget >/dev/null 2>&1; then
      if ! wget -qO "${tmp_file}" \
        --timeout="${DO_AGENT_DOWNLOAD_WGET_TIMEOUT}" \
        --tries="${DO_AGENT_DOWNLOAD_WGET_TRIES}" \
        "${DO_AGENT_INSTALL_URL}"; then
        echo "WARN: failed to download do-agent install script, continuing without it" >&2
        exit 1
      fi
    else
      echo "WARN: neither curl nor wget available, skipping do-agent install" >&2
      exit 1
    fi

    if [ ! -s "${tmp_file}" ]; then
      echo "WARN: downloaded do-agent install script is empty, skipping" >&2
      exit 1
    fi

    # Upstream install.sh uses bash-only options (e.g. pipefail); /bin/sh is often dash on Debian/Ubuntu.
    if command -v bash >/dev/null 2>&1; then
      bash "${tmp_file}" || {
        echo "WARN: do-agent installation failed, continuing without it" >&2
      }
    else
      echo "WARN: bash not found; do-agent install script requires bash, skipping" >&2
    fi
  )
  return 0
}

# leave this last to prevent any partial executions
main
