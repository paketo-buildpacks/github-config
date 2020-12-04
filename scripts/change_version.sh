#!/usr/bin/env bash

set -eu
set -o pipefail

readonly ROOTDIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )/.." && pwd )"

# shellcheck source=SCRIPTDIR/.util/print.sh
source "${ROOTDIR}/scripts/.util/print.sh"

function main() {
  local repo version token

  while [[ "${#}" != 0 ]]; do
    case "${1}" in
      --help|-h)
        shift 1
        usage
        exit 0
        ;;

      --repo|-r)
        repo="${2}"
        shift 2
        ;;

      --version|-v)
        version="${2}"
        shift 2
        ;;

      --token|-t)
        token="${2}"
        shift 2
        ;;

      *)
        util::print::error "unknown argument \"${1}\""
    esac
  done

  change_draft_release_version "${repo}" "${version}" "${token}"
  generate_new_draft_release "${repo}" "${version}" "${token}"
}

function usage() {
  cat <<-USAGE
change_version.sh [OPTIONS]

Changes the version of the chosen buildpack and creates a draft release with that version

OPTIONS
  --help     -h  prints the command usage
  --repo     -r  name of the GitHub repository of the buildpack to version bump <org>/<name>
  --version  -v  semver valid version to create a new draft release of
  --token    -t  GitHub token used to check the repository
USAGE
}

function change_draft_release_version() {
  local repo version token draft_id
  repo="${1}"
  version="${2}"
  token="${3}"

  util::print::info "getting current draft release..."
  draft_id=$(curl --silent -XGET \
    "https://api.github.com/repos/${repo}/releases" \
    --header "Accept: application/vnd.github.v3+json" \
    --header "Authorization: token ${token}" \
    | jq '.[] | select(.draft==true)' \
    | jq -r .id
  )

  if [[ -z "${draft_id}" ]]; then
    util::print::error "this script relies on the repo having an existing draft release"
  else
    util::print::info "editing draft release..."
    curl --silent -XPATCH \
      "https://api.github.com/repos/${repo}/releases/${draft_id}" \
      --header "Accept: application/vnd.github.v3+json" \
      --header "Authorization: token ${token}" \
      -d "{\"tag_name\": \"${version}\", \"name\": \"${version}\"}"
  fi
}

function generate_new_draft_release() {
  local repo version token
  repo="${1}"
  version="${2}"
  token="${3}"

  util::print::info "kicking off release process..."
  curl \
    -X POST \
    --header "Accept: application/vnd.github.v3+json" \
    --header "Authorization: token ${token}" \
    "https://api.github.com/repos/${repo}/dispatches" \
    -d '{"event_type":"version-bump"}'
}

main "${@:-}"
