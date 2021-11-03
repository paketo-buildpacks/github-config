#!/bin/bash
set -eu
set -o pipefail

function main() {
  local repo number patchfiles

  while [ "${#}" != 0 ]; do
    case "${1}" in
      --repo)
        repo="${2}"
        shift 2
        ;;

      --number)
        number="${2}"
        shift 2
        ;;

      --patchfiles)
        patchfiles="${2}"
        shift 2
        ;;

      "")
        shift
        ;;

      *)
        echo "unknown argument \"${1}\""
        exit 1
    esac
  done

  set +e
  gh auth status
  retVal=$?
  if [ $retVal -ne 0 ]; then
    echo "No Github credentials provided. Skipping labeling."
    echo "::set-output name=label::"
    exit 0
  fi
  set -e

  # Check that the files-changed API endpoint is valid for the given parameters
  gh api /repos/"${repo}"/pulls/"${number}"/files --jq '.[].filename' > /dev/null

  if ! [ -f "${patchfiles}" ]; then
    echo "${patchfiles} does not exist."
    exit 1
  fi

  while read -r changed; do
    echo "File changed: ${changed}"
    # scan through an allow list. does each element of the changed files match something in the allow list?
    safe=0
    while read -r file; do
      if [[ "${changed}" =~ ${file} ]]; then
        echo "${changed} is on the patch allowlist"
        safe=1
        break
      fi
    done < "${patchfiles}"

    if [ "${safe}" -eq "0" ]; then
      echo "Files changed that aren't on the patch allowlist"
      echo "::set-output name=label::"
      exit 0
    fi
  done < <(gh api /repos/"${repo}"/pulls/"${number}"/files --jq '.[].filename')

  echo "All changes are patches."
  echo "::set-output name=label::patch"
}

main "${@:-}"
