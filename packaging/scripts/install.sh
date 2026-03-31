#!/bin/sh
#   curl -sSL https://triton.sfo3.cdn.digitaloceanspaces.com/install.sh | sudo bash
#   wget -qO- https://triton.sfo3.cdn.digitaloceanspaces.com/install.sh | sudo bash

set -u

REPO_DOMAIN="triton.sfo3.cdn.digitaloceanspaces.com"
REPO_HOST="https://${REPO_DOMAIN}"
REPO_GPG_KEY=${REPO_HOST}/gpg.key

branch="do-obsd-preview"

RETRY_CRON_SCHEDULE=/etc/cron.hourly
RETRY_CRON=${RETRY_CRON_SCHEDULE}/do-obsd-install

dist="unknown"
exit_status=0
no_retry="false"
repo_name=do-obsd
deb_list=/etc/apt/sources.list.d/${repo_name}.list
deb_pref=/etc/apt/preferences.d/${repo_name}.pref
deb_keyfile=/usr/share/keyrings/${repo_name}-keyring.gpg
rpm_repo=/etc/yum.repos.d/${repo_name}.repo

main() {
  [ "$(id -u)" != "0" ] &&
    abort "This script must be executed as root."

  trap 'exit_status=$?; script_cleanup; exit $exit_status' EXIT

  check_do
  check_dist
  check_arch

  case "${dist}" in
  debian | ubuntu)
    i=1
    until [ "$i" -ge 6 ]; do
      echo "Installing do-obsd, attempt ${i}"
      install_apt
      exit_status=$?
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

  cat <<'EOF' >"${RETRY_CRON}"
#!/bin/sh
tmp_file=$(mktemp -t do_obsd.install.XXXXXX)
trap "rm -f \"${tmp_file}\"" EXIT
url="https://triton.sfo3.cdn.digitaloceanspaces.com/install.sh"
log_file="/var/log/do-obsd.install.log"

if command -v curl >/dev/null 2>&1; then
  if ! curl -sSL "${url}" -o "${tmp_file}"; then
    now=$(date +"%T")
    echo "Retry at: ${now} - failed to download install script with curl" >> "${log_file}"
    exit 1
  fi
elif command -v wget >/dev/null 2>&1; then
  if ! wget -qO "${tmp_file}" "${url}"; then
    now=$(date +"%T")
    echo "Retry at: ${now} - failed to download install script with wget" >> "${log_file}"
    exit 1
  fi
else
  now=$(date +"%T")
  echo "Retry at: ${now} - neither curl nor wget is installed; cannot download install script" >> "${log_file}"
  exit 1
fi

now=$(date +"%T")
echo "Retry at: ${now}" >> "${log_file}"
/bin/sh "${tmp_file}" >> "${log_file}" 2>&1
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

  echo "Setting up do-obsd apt repository..."
  install_deps "deb"

  echo "Importing GPG public key"
  wget -qO- "${REPO_GPG_KEY}" | gpg --dearmor >"${deb_keyfile}"
  echo "deb [signed-by=${deb_keyfile}] ${REPO_HOST}/apt/${branch} main main" >"${deb_list}"
  cat <<-EOF >${deb_pref}
	Package: *
	Pin: origin ${REPO_DOMAIN}
	Pin-Priority: 100
	EOF

  echo "Installing do-obsd"
  apt-get -qq update
  apt-get -qq --fix-missing install -y do-obsd
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

check_arch() {
  echo "Checking architecture support..."
  if [ "$(uname -m)" != "x86_64" ]; then
    not_supported
  fi
  echo "OK"
}

check_do() {
  echo "Verifying machine compatibility..."
  dmi_bios_file="/sys/devices/virtual/dmi/id/bios_vendor"
  if [ -f "${dmi_bios_file}" ]; then
    read -r sys_vendor <${dmi_bios_file}
  else
    sys_vendor=$(dmidecode -s bios-vendor)
  fi
  if ! [ "$sys_vendor" = "DigitalOcean" ]; then
    cat <<-EOF

		The DigitalOcean Observability Supervisor is only supported on DigitalOcean machines.

		If you are seeing this message on an older droplet, you may need to power-off
		and then power-on at http://cloud.digitalocean.com. After power-cycling,
		please re-run this script.

		EOF
    exit 1
  fi
  echo "OK"
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

# do-agent upstream installer: HTTPS and host are fixed; SHA256 must match the file
# at this URL (update DO_AGENT_INSTALL_SHA256 when DigitalOcean changes install.sh).
DO_AGENT_INSTALL_HOST="repos.insights.digitalocean.com"
DO_AGENT_INSTALL_PATH="/install.sh"
DO_AGENT_INSTALL_URL="https://${DO_AGENT_INSTALL_HOST}${DO_AGENT_INSTALL_PATH}"
DO_AGENT_INSTALL_SHA256="16ed4f1124ea4c9cf6507ed040b36214aee3077c8949cf2fb4aca2b477f2346a"

validate_do_agent_install_url() {
  case "${DO_AGENT_INSTALL_URL}" in
  "https://${DO_AGENT_INSTALL_HOST}${DO_AGENT_INSTALL_PATH}")
    return 0
    ;;
  *)
    echo "WARN: do-agent install URL must be HTTPS on ${DO_AGENT_INSTALL_HOST} only, skipping" >&2
    return 1
    ;;
  esac
}

verify_do_agent_installer_checksum() {
  _path="$1"
  _actual=""
  if command -v sha256sum >/dev/null 2>&1; then
    _actual=$(sha256sum "${_path}" | awk '{print $1}')
  elif command -v shasum >/dev/null 2>&1; then
    _actual=$(shasum -a 256 "${_path}" | awk '{print $1}')
  else
    echo "WARN: sha256sum and shasum unavailable, cannot verify do-agent installer, skipping" >&2
    return 1
  fi
  if [ "${_actual}" != "${DO_AGENT_INSTALL_SHA256}" ]; then
    echo "WARN: do-agent installer SHA256 mismatch, skipping (supply-chain check failed)" >&2
    return 1
  fi
  return 0
}

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

  if ! validate_do_agent_install_url; then
    return 0
  fi

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
      if ! curl -fsSL "${DO_AGENT_INSTALL_URL}" -o "${tmp_file}"; then
        echo "WARN: failed to download do-agent install script, continuing without it" >&2
        exit 1
      fi
    elif command -v wget >/dev/null 2>&1; then
      if ! wget -qO "${tmp_file}" "${DO_AGENT_INSTALL_URL}"; then
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

    if ! verify_do_agent_installer_checksum "${tmp_file}"; then
      exit 1
    fi

    /bin/sh "${tmp_file}" || {
      echo "WARN: do-agent installation failed, continuing without it" >&2
    }
  )
  return 0
}

# leave this last to prevent any partial executions
main
