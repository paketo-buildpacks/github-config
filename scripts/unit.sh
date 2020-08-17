#!/bin/bash

set -e
set -u
set -o pipefail

readonly ROOT_DIR="$(cd "$(dirname "${0}")/.." && pwd)"

# shellcheck source=SCRIPTDIR/../language-family/scripts/.util/tools.sh
source "${ROOT_DIR}/language-family/scripts/.util/tools.sh"

# shellcheck source=SCRIPTDIR/.util/print.sh
source "${ROOT_DIR}/scripts/.util/print.sh"

function tools::install() {
  util::tools::jam::install \
      --directory "${ROOT_DIR}/.bin"
}

function main() {
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

  tools::install

  while read -r directory; do
    pushd "${directory}" > /dev/null || return
      go test -v -count=1 .
    popd > /dev/null || return
  done < <(find "${ROOT_DIR}" -type d -name entrypoint ! -path '*/.git/*')
}

function usage() {
  cat <<-USAGE
unit.sh [OPTIONS]

Runs the unit test suite.

OPTIONS
  --help  -h  prints the command usage
USAGE
}

main "${@:-}"
