#!/bin/bash
# vim: noexpandtab

set -ue

SVC_NAME="do-obsd"

REPO_HOST=""
PKG_PATTERN=""
ARCH="x86_64"
LATEST_VER="-"
LOCAL_VER=""

main() {
  delay=$((RANDOM % 900))
  echo "Waiting ${delay} seconds"
  sleep ${delay}

  check_arch
  if command -v apt-get >/dev/null 2>&1; then
    platform="deb"
    do_update=update_deb
  elif command -v yum >/dev/null 2>&1; then
    platform="rpm"
    do_update=update_rpm
  else
    not_supported
  fi
  prepare "${platform}"
  find_latest_pkg "${platform}"
  if [ "${LOCAL_VER}" = "${LATEST_VER}" ]; then
    echo "No need to update"
    exit 0
  fi
  ${do_update}
}

update_deb() {
  echo "Updating ${SVC_NAME} deb package"
  export DEBIAN_FRONTEND="noninteractive"
  apt-get -qq update \
    -o Dir::Etc::SourceParts=/dev/null \
    -o APT::Get::List-Cleanup=no \
    -o Dir::Etc::SourceList="sources.list.d/${SVC_NAME}.list"
  apt-get \
    -o Dpkg::Options::="--force-confdef" \
    -o Dpkg::Options::="--force-confold" \
    -qq install -y --only-upgrade ${SVC_NAME}
}

update_rpm() {
  echo "Updating ${SVC_NAME} rpm package"
  yum -q -y --disablerepo="*" --enablerepo="${SVC_NAME}" makecache
  yum -q -y update ${SVC_NAME}
}

check_arch() {
  echo -n "Checking architecture support..."
  if [ "$(uname -m)" != "$ARCH" ]; then
    not_supported
  fi
  echo "OK"
}

prepare() {
  echo "Preparing to check for update"
  platform=${1:-}
  [ -z "${platform}" ] && abort "Destination repository is required. Usage: prepare <platform>"
  case "${platform}" in
  rpm)
    LOCAL_VER=$(rpm -q ${SVC_NAME} --qf '%{VERSION}')
    url=$(grep baseurl <"/etc/yum.repos.d/${SVC_NAME}.repo" | cut -f 2 -d=)
    url=$(echo "${url}/${SVC_NAME}." | sed -e "s|\$basearch|${ARCH}|g")
    ;;
  deb)
    LOCAL_VER=$(dpkg -s ${SVC_NAME} | grep Version | cut -f 2 -d: | tr -d '[:space:]')
    url=$(cut -f 3 -d' ' <"/etc/apt/sources.list.d/${SVC_NAME}.list")
    url="${url}/pool/main/main/d/${SVC_NAME}/${SVC_NAME}_"
    ;;
  esac
  REPO_HOST=$(echo "${url}" | grep "/" | cut -d"/" -f1-3)
  PKG_PATTERN=$(echo "${url}" | grep "/" | cut -d"/" -f4-)
  echo "Package Host: ${REPO_HOST}"
  echo "Package Path: ${PKG_PATTERN}"
  echo "Local Version:${LOCAL_VER}"
}

find_latest_pkg() {
  platform=${1:-}
  [ -z "${platform}" ] && abort "Destination repository is required. Usage: find_latest_pkg <platform>"

  echo "Checking Latest Version..."
  case "${platform}" in
  rpm)
    repo_tree=$(curl -sSL "${REPO_HOST}")
    ;;
  deb)
    repo_tree=$(wget -qO- "${REPO_HOST}")
    ;;
  esac
  files=$(printf '%s\n' "${repo_tree}" | sed -n 's/.*Key>\([^<]*\)<.*/\1/p' | grep -F "${PKG_PATTERN}" | tr ' ' '\n')
  sorted_files=$(printf '%s\n' "${files}" | sort -V)
  LATEST_VER=$(printf '%s\n' "${sorted_files}" | tail -n 1 | sed -n 's/.*\([0-9][0-9A-Za-z.~+-]*\).*/\1/p')
  echo "Latest package:${LATEST_VER}"
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

main
