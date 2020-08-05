#!/usr/bin/env bash

set -eu
set -o pipefail

readonly ROOTDIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )/.." && pwd )"

function main() {
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
