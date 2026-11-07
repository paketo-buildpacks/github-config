#!/usr/bin/env bash

set -eu
set -o pipefail

readonly PROGDIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# shellcheck source=SCRIPTDIR/.util/print.sh
source "${PROGDIR}/.util/print.sh"

function sanity::check() {
  local dir
  dir="${1}"
  shift 1

  while [[ "${#}" != 0 ]]; do
    case "${1}" in
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

  sanity::check::rule::directories "${dir}"
}

function usage() {
  cat <<-USAGE
sanity.sh [OPTIONS]

Checks if the contents of builder/ implementation/ language-family/ and stack/
follow rules.

OPTIONS
  --help  -h  prints the command usage
USAGE
}

# Rule: all children of implemenation/ & language-family/ must be directories
function sanity::check::rule::directories() {
  local dir
  dir="${1}"

  for cnbdir in 'builder' 'implementation' 'language-family' 'stack' ; do
    if [[ ! -d "${dir}/${cnbdir}" ]]; then
      echo "${cnbdir} dir not found"
      exit  1
    fi

    if [[ -n "$(find "${dir}/${cnbdir}" -maxdepth 1 \! -type d)" ]]; then
      echo "All files in ${cnbdir}/ must be directories. Exiting."
      exit 1
    fi
  done
}

if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
  sanity::check "$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)" "${@:-}"
fi
