#!/usr/bin/env bash

set -eu
set -o pipefail

readonly PROGDIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# shellcheck source=SCRIPTDIR/.util/tools.sh
source "${PROGDIR}/.util/tools.sh"

# shellcheck source=SCRIPTDIR/.util/print.sh
source "${PROGDIR}/.util/print.sh"

function main() {
  local directory jam_version libpak_tools_version tools_config token
  directory="${TOOLS_DIRECTORY:-}"
  jam_version="${JAM_VERSION:-}"
  libpak_tools_version="${LIBPAK_TOOLS_VERSION:-}"
  tools_config="${TOOLS_CONFIG:-}"
  token="${GITHUB_TOKEN:-}"

  while [[ "${#}" != 0 ]]; do
    case "${1}" in
      --directory)
        directory="${2}"
        shift 2
        ;;

      --jam-version)
        jam_version="${2}"
        shift 2
        ;;

      --libpak-tools-version)
        libpak_tools_version="${2}"
        shift 2
        ;;

      --tools-config)
        tools_config="${2}"
        shift 2
        ;;

      --token)
        token="${2}"
        shift 2
        ;;

      --help|-h)
        shift 1
        usage
        exit 0
        ;;

      "")
        # skip if the argument is empty
        shift 1
        ;;

      *)
        util::print::error "unknown argument \"${1}\""
    esac
  done

  directory="${directory:-${HOME}/bin}"

  jam_version="$(util::tools::version::resolve "jam" "${jam_version}" "${tools_config}")"
  libpak_tools_version="$(util::tools::version::resolve "libpaktools" "${libpak_tools_version}" "${tools_config}")"

  if [[ -z "${jam_version}" && -z "${libpak_tools_version}" ]]; then
    util::print::error "unable to determine a tool version from inputs or ${tools_config:-the tools config file}"
  fi

  tools::install "${directory}" "${token}" "${jam_version}" "${libpak_tools_version}"

  if [[ -n "${GITHUB_PATH:-}" ]]; then
    echo "${directory}" >> "${GITHUB_PATH}"
  fi

  if [[ -n "${GITHUB_OUTPUT:-}" ]]; then
    echo "directory=${directory}" >> "${GITHUB_OUTPUT}"
  fi
}

function usage() {
  cat <<-USAGE
setup.sh [OPTIONS]

Installs jam and libpak-tools.

OPTIONS
  --directory <path>               Directory to install the tools into (default: \${HOME}/bin)
  --jam-version <version>          Version of jam to install. Overrides the tools config file.
  --libpak-tools-version <version> Version of libpak-tools to install. Overrides the tools config file.
  --tools-config <path>            Path to a JSON file containing tool versions.
  --token <token>                  Token used to download assets from GitHub (optional)
  --help  -h                       prints the command usage
USAGE
}

function tools::install() {
  local directory token jam_version libpak_tools_version
  directory="${1}"
  token="${2}"
  jam_version="${3}"
  libpak_tools_version="${4}"

  if [[ -n "${jam_version}" ]]; then
    util::tools::jam::install \
      --directory "${directory}" \
      --version "${jam_version}" \
      --token "${token}"
  else
    util::print::info "Skipping jam: no version provided or found"
  fi

  if [[ -n "${libpak_tools_version}" ]]; then
    util::tools::libpak-tools::install \
      --directory "${directory}" \
      --version "${libpak_tools_version}" \
      --token "${token}"
  else
    util::print::info "Skipping libpak-tools: no version provided or found"
  fi
}

main "${@:-}"
