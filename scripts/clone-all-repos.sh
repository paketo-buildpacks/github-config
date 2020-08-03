#!/usr/bin/env bash

set -e
set -u
set -o pipefail

readonly ROOTDIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )/.." && pwd )"

extra_repos=$(cat <<-END
paketo-buildpacks/packit
paketo-buildpacks/occam
paketo-buildpacks/github-config
paketo-buildpacks/paketo-website
END
)

function clone_all_repos() {
  pushd "${HOME}/workspace" > /dev/null
    for datafile in 'implementation-cnbs' 'language-family-cnbs' ; do
      while read -r line; do
       clone_if_not_exist "${line}"
      done < "${ROOTDIR}/.github/data/${datafile}"
    done

    for line in ${extra_repos};
    do
       clone_if_not_exist "${line}"
    done <<< "${extra_repos}"
  popd > /dev/null
}

function clone_if_not_exist() {
  local repo
  repo="${1}"
  if [[ ! -d $(echo "${repo}" | cut -d'/' -f2) ]]; then
    echo "Cloning ${repo}"
    git clone "git@github.com:${repo}.git"
  else
    echo "${line} already cloned"
  fi
  echo ""
}


clone_all_repos "${@:-}"
