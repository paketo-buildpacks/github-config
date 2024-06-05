#!/bin/bash
set -euo pipefail
shopt -s inherit_errexit

function main() {
  local repo number author patchfiles

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

      --author)
        author="${2}"
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

  if [[ "${author}" == "dependabot[bot]" ]]; then
    echo "PR author is dependabot. Labeling as patch."
    echo "label=patch" >> "$GITHUB_OUTPUT"
    exit 0
  fi

  set +e
  gh auth status
  retVal=$?
  if [ $retVal -ne 0 ]; then
    echo "No Github credentials provided. Skipping labeling."
    echo "label=" >> "$GITHUB_OUTPUT"
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
    # scan through an allow list. does each element of the changed files match
    # something in the allow list?
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
      echo "label=" >> "$GITHUB_OUTPUT"
      exit 1
    fi
  done < <(gh api /repos/"${repo}"/pulls/"${number}"/files --jq '.[].filename')

  echo "All changes are patches."
  echo "label=patch" >> "$GITHUB_OUTPUT"
}

main "${@:-}"
