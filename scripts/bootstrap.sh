#!/usr/bin/env bash

set -eu
set -o pipefail

readonly ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

# shellcheck source=SCRIPTDIR/.util/print.sh
source "${ROOT_DIR}/scripts/.util/print.sh"

# shellcheck source=SCRIPTDIR/sanity.sh
source "${ROOT_DIR}/scripts/sanity.sh"

function main() {
  local target repo_type

  while [[ "${#}" != 0 ]]; do
    case "${1}" in
      --target)
        target="${2}"
        shift 2
        ;;

      --repo-type)
        repo_type="${2}"
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

  if [[ -z "${repo_type:-}" ]]; then
    usage
    echo
    util::print::error "--repo-type is a required flag"
  fi

  sanity::check "${ROOT_DIR}"
  bootstrap "${target}" "${repo_type}"
}

function usage() {
  cat <<-USAGE
bootstrap.sh --target <target> --repo-type <repo-type> [OPTIONS]

Bootstraps a repository with github configuration and scripts.

OPTIONS
  --repo-type <repo-type>  type of repo (implementation|language-family|builder)
  --help  -h               prints the command usage
  --target <target>        path to a buildpack repository
USAGE
}

function bootstrap() {
  local target repo_type
  target="${1}"
  repo_type="${2}"

  if [[ ! -d "${target}" ]]; then
    util::print::error "cannot bootstrap: \"${target}\" does not exist"
  fi

  if [[ "${repo_type}" != "implementation" && "${repo_type}" != "language-family" && "${repo_type}" != "builder" ]]; then
    util::print::error "cannot bootstrap: \"${repo_type}\" is not a valid repo type"
  fi

  cp -pR "${ROOT_DIR}/${repo_type}/." "${target}"
}

main "${@:-}"
