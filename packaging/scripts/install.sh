#!/bin/sh
#   curl -sSL https://marlin.nyc3.cdn.digitaloceanspaces.com/install.sh | sudo bash
#   wget -qO- https://marlin.nyc3.cdn.digitaloceanspaces.com/install.sh | sudo bash

set -u

REPO_DOMAIN="marlin.nyc3.cdn.digitaloceanspaces.com"
REPO_HOST="https://${REPO_DOMAIN}"
REPO_GPG_KEY=${REPO_HOST}/gpg.key

branch="do-obsd-preview"

DOAGENT_REPO_HOST="https://repos.insights.digitalocean.com"
DOAGENT_GPG_KEY="${DOAGENT_REPO_HOST}/sonar-agent.asc"

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

  trap 'script_cleanup; exit $exit_status' EXIT

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
trap "rm -f ${tmp_file}" EXIT
url="https://marlin.nyc3.cdn.digitaloceanspaces.com/install.sh"
install_script=$(curl -sSL "${url}" || wget -qO- "${url}")
echo "${install_script}" > ${tmp_file}
now=$(date +"%T")
echo "Retry at: ${now}" > /var/log/do-obsd.install.log
/bin/bash ${tmp_file} >> /var/log/do-obsd.install.log 2>&1
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

ensure_doagent_apt() {
  if dpkg -s do-agent >/dev/null 2>&1; then
    return 0
  fi

  echo "do-agent not found, configuring do-agent repository..."
  doagent_keyfile=/usr/share/keyrings/digitalocean-agent-keyring.gpg
  doagent_list=/etc/apt/sources.list.d/digitalocean-agent.list

  wget -qO- "${DOAGENT_GPG_KEY}" | gpg --dearmor >"${doagent_keyfile}"
  echo "deb [signed-by=${doagent_keyfile}] ${DOAGENT_REPO_HOST}/apt/do-agent main main" >"${doagent_list}"
  apt-get -qq update -o Dir::Etc::SourceParts=/dev/null -o APT::Get::List-Cleanup=no -o Dir::Etc::SourceList="sources.list.d/digitalocean-agent.list"
}

ensure_doagent_yum() {
  if rpm -q do-agent >/dev/null 2>&1; then
    return 0
  fi

  echo "do-agent not found, configuring do-agent repository..."
  doagent_repo=/etc/yum.repos.d/digitalocean-agent.repo
  cat <<-REPO >${doagent_repo}
	[digitalocean-agent]
	name=DigitalOcean Agent
	baseurl=${DOAGENT_REPO_HOST}/yum/do-agent/\$basearch
	repo_gpgcheck=0
	gpgcheck=1
	enabled=1
	gpgkey=${DOAGENT_GPG_KEY}
	sslverify=0
	sslcacert=/etc/pki/tls/certs/ca-bundle.crt
	metadata_expire=300
	REPO

  yum --disablerepo="*" --enablerepo="digitalocean-agent" makecache
}

install_apt() (
  set -e
  export DEBIAN_FRONTEND=noninteractive

  echo "Setting up do-obsd apt repository..."
  install_deps "deb"

  ensure_doagent_apt

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

  ensure_doagent_yum

  cat <<-EOF >${rpm_repo}
	[${repo_name}]
	name=DigitalOcean Observability Supervisor
	baseurl=${REPO_HOST}/yum/${branch}/\$basearch
	repo_gpgcheck=0
	gpgcheck=1
	enabled=1
	gpgkey=${REPO_GPG_KEY}
	sslverify=0
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

# leave this last to prevent any partial executions
main
