#!/usr/bin/env bash

set -eu
set -o pipefail

readonly PROG_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly ROOT_DIR="$(cd "${PROG_DIR}/.." && pwd)"
readonly STACK_DIR="${ROOT_DIR}/stack"
readonly BIN_DIR="${ROOT_DIR}/.bin"
readonly BUILD_DIR="${ROOT_DIR}/build"

# shellcheck source=SCRIPTDIR/.util/tools.sh
source "${PROG_DIR}/.util/tools.sh"

# shellcheck source=SCRIPTDIR/.util/print.sh
source "${PROG_DIR}/.util/print.sh"

if [[ $BASH_VERSINFO -lt 4 ]]; then
  util::print::error "Before running this script please update Bash to v4 or higher (e.g. on OSX: \$ brew install bash)"
fi

function main() {
  local unbuffered secrets

  unbuffered="false"

  while [[ "${#}" != 0 ]]; do
    case "${1}" in
      --help|-h)
        shift 1
        usage
        exit 0
        ;;

      --unbuffered)
        unbuffered="true"
        shift 1
        ;;

      --secret)
        secrets+=("${2}")
        shift 2
        ;;

      "")
        # skip if the argument is empty
        shift 1
        ;;

      *)
        util::print::error "unknown argument \"${1}\""
    esac
  done

  mkdir -p "${BUILD_DIR}"

  tools::install
  stack::create "${unbuffered}" ${secrets[@]}
}

function usage() {
  cat <<-USAGE
create.sh [OPTIONS]

Creates the stack using the descriptor, build and run Dockerfiles in
the repository.

OPTIONS
  --help       -h   prints the command usage
  --unbuffered      do not buffer image contents into memory for fast access
  --secret          provide a secret in the form key=value. Use flag multiple times to provide multiple secrets
USAGE
}


function tools::install() {
  util::tools::jam::install \
    --directory "${BIN_DIR}"
}

function stack::create() {
  local unbuffered secrets

  unbuffered="${1}"
  shift 1
  secrets=("${@}")

  if [[ "${unbuffered}" == "true" ]]; then
    echo "Running in unbuffered mode - this may take substantially longer"
    echo
  fi

  args=(
      --config "${STACK_DIR}/stack.toml"
      --build-output "${BUILD_DIR}/build.oci"
      --run-output "${BUILD_DIR}/run.oci"
      --unbuffered="${unbuffered}"
    )

  for secret in "${secrets[@]}"; do
    args+=("--secret" "${secret}")
  done

  jam create-stack "${args[@]}"
}

main "${@:-}"
