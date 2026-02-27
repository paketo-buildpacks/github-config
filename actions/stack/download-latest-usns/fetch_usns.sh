#!/usr/bin/env bash

set -eu
set -o pipefail

function main() {
  local distro=""
  local limit=""
  local offset=""
  local usns_output_path=""
  while [[ "${#}" != 0 ]]; do
    case "${1}" in
      --help|-h)
        shift 1
        usage
        exit 0
        ;;

      --distro|-d)
        distro="${2}"
        shift 2
        ;;

      --limit|-l)
        limit="${2}"
        shift 2
        ;;

      --offset|-o)
        offset="${2}"
        shift 2
        ;;

      --usns-output-path|-u)
        usns_output_path="${2}"
        shift 2
        ;;

      "")
        shift 1
        ;;

      *)
        echo "unknown argument \"${1}\"" >&2
        usage
        exit 1
        ;;
    esac
  done

  if [[ -z "${distro:-}" ]]; then
    echo "error: --distro is required" >&2
    usage
    exit 1
  fi

  if [[ -z "${usns_output_path:-}" ]]; then
    echo "error: --usns-output-path is required" >&2
    usage
    exit 1
  fi

  local url="https://ubuntu.com/security/notices.json?release=${distro}"

  if [[ -n "${limit:-}" ]]; then
    url="${url}&limit=${limit}"
  fi

  if [[ -n "${offset:-}" ]]; then
    url="${url}&offset=${offset}"
  fi

  mkdir -p "$(dirname "${usns_output_path}")"
  curl -sSfL "${url}" > "${usns_output_path}" || { echo "error: failed to fetch notices for distro ${distro}" >&2; exit 1; }
}

function usage() {
  cat <<-ENDUSAGE
fetch_usns.sh [OPTIONS]

Fetches Ubuntu Security Notices (USNs) JSON for the given distro from ubuntu.com and saves it to the given path.

USAGE
  ./scripts/fetch_usns.sh --distro jammy --limit 20 --offset 0 --usns-output-path ./jammy-usns.json

OPTIONS
  --distro <name>     -d <name>  Ubuntu distro to fetch (e.g. bionic, focal, jammy, noble). Required.
  --limit   <n>       -l <n>     Maximum number of notices to fetch (optional). When omitted, the API returns 20 items.
  --offset  <n>       -o <n>     Number of notices to skip (optional). When omitted, the API starts from the first notice.
  --usns-output-path <path>    -u <path>  Path to output USNs JSON file. Required.
  --help              -h         prints the command usage
ENDUSAGE
}

main "${@:-}"
