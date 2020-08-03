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
  local line
  pushd "${HOME}/workspace" > /dev/null
    for datafile in 'implementation-cnbs' 'language-family-cnbs' ; do
      while read -r line; do
       clone_or_pull "${line}"
      done < "${ROOTDIR}/.github/data/${datafile}"
    done

    for line in ${extra_repos};
    do
       clone_or_pull "${line}"
    done <<< "${extra_repos}"
  popd > /dev/null
}

function clone_or_pull() {
  local repo name org
  repo="${1}"
  org=$(echo "${repo}" | cut -d '/' -f1)
  name=$(echo "${repo}" | cut -d '/' -f2)

  mkdir -p "${org}"
  pushd "${org}" > /dev/null
    if [[ ! -d "${name}" ]]; then
      echo "Cloning ${repo}"
      git clone "git@github.com:${repo}.git"
    else
      echo "${repo} already cloned, updating"
      pushd "${name}" > /dev/null
        git co master ## Change to main when we switch all of our repos over
        git pull
      popd > /dev/null
    fi
    echo ""
  popd > /dev/null
}


clone_all_repos "${@:-}"
