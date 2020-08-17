#!/usr/bin/env bash

set -eu
set -o pipefail

readonly ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

# shellcheck source=SCRIPTDIR/.util/print.sh
source "${ROOT_DIR}/scripts/.util/print.sh"

# shellcheck source=SCRIPTDIR/sanity.sh
source "${ROOT_DIR}/scripts/sanity.sh"

function main() {
  local target buildpack_type

  while [[ "${#}" != 0 ]]; do
    case "${1}" in
      --target)
        target="${2}"
        shift 2
        ;;

      --buildpack-type)
        buildpack_type="${2}"
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

  if [[ -z "${target:-}" ]]; then
    usage
    echo
    util::print::error "--target is a required flag"
  fi

  if [[ -z "${buildpack_type:-}" ]]; then
    usage
    echo
    util::print::error "--buildpack-type is a required flag"
  fi

  sanity::check "${ROOT_DIR}"
  bootstrap "${target}" "${buildpack_type}"
}

function usage() {
  cat <<-USAGE
bootstrap.sh --target <target> --buildpack-type <buildpack-type> [OPTIONS]

Bootstraps a buildpack repository with github configuration and scripts.

OPTIONS
  --buildpack-type <buildpack-type>  type of buildpack (implementation|language-family)
  --help  -h                         prints the command usage
  --target <target>                  path to a buildpack repository
USAGE
}

function bootstrap() {
  local target buildpack_type
  target="${1}"
  buildpack_type="${2}"

  if [[ ! -d "${target}" ]]; then
    util::print::error "cannot bootstrap: \"${target}\" does not exist"
  fi

  if [[ "${buildpack_type}" != "implementation" && "${buildpack_type}" != "language-family" ]]; then
    util::print::error "cannot bootstrap: \"${buildpack_type}\" is not a valid buildpack type"
  fi

  cp -pR "${ROOT_DIR}/${buildpack_type}/." "${target}"
}

main "${@:-}"
