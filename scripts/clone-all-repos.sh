#!/usr/bin/env bash

set -eu
set -o pipefail

readonly ROOTDIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )/.." && pwd )"

# shellcheck source=SCRIPTDIR/.util/print.sh
source "${ROOTDIR}/scripts/.util/print.sh"

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

  clone_all
}

function usage() {
  cat <<-USAGE
clone-all-repos.sh [OPTIONS]

Clones all project repositories into a set of organization-scopes workspace directories.

OPTIONS
  --help  -h  prints the command usage
USAGE
}

function clone_all() {
  local repos line
  repos="$(
    cat "${ROOTDIR}/.github/data/implementation-cnbs"
    cat "${ROOTDIR}/.github/data/language-family-cnbs"
    cat <<-EOF | xargs -n1
      paketo-buildpacks/github-config
      paketo-buildpacks/occam
      paketo-buildpacks/packit
      paketo-buildpacks/paketo-website
      paketo-buildpacks/rfcs
      paketo-buildpacks/samples
		EOF
  )"

  while read -r line; do
    clone_or_pull "${line}"
  done < <( echo "${repos}" | sort)
}

function clone_or_pull() {
  local repo
  repo="${1}"

  local path
  path="${HOME}/workspace/${repo}"

  mkdir -p "$(dirname "${path}")"

  if [[ ! -d "${path}" ]]; then
    echo "Cloning ${repo}"
    git clone "git@github.com:${repo}.git" "${path}"
  else
    echo "${repo} already cloned, updating"

    git -C "${path}" checkout main
    git -C "${path}" pull --rebase --autostash
  fi

  echo
}

main "${@:-}"
