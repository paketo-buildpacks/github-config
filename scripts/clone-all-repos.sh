#!/bin/bash

set -e
set -u
set -o pipefail

PROGDIR="$(cd "$(dirname "${0}")" && pwd)"
readonly PROGDIR

# shellcheck source=SCRIPTDIR/.util/print.sh
source "${PROGDIR}/.util/print.sh"

function main() {
  local workspace token

  while [[ "${#}" != 0 ]]; do
    case "${1}" in
      --help|-h)
        shift 1
        usage
        exit 0
        ;;

      --workspace)
        workspace="${2}"
        shift 2
        ;;

      --token)
        token="${2}"
        shift 2
        ;;

      "")
        # skip if the argument is empty
        shift 1
        ;;

      *)
        util::print::error "unknown argument \"${1}\""
    esac
  done

  if [[ -z "${token:-}" ]]; then
    util::print::error "--token is a required flag"
  fi

  if [[ -z "${workspace:-}" ]]; then
    workspace="${HOME}/workspace"
  fi

  repos::clone::all "${workspace}" "${token}"

  util::print::green "All repos cloned. Look around in ${workspace}"
}

function usage() {
  cat <<-USAGE
clone-all-repos.sh [OPTIONS]

Clones relevant Paketo Buildpacks and Paketo Community repos into a workspace.

OPTIONS
  --help              -h  prints the command usage
  --workspace <path>      directory where repos are cloned (defaults to "${HOME}/workspace")
  --token <token>         GitHub token used to fetch repository details
USAGE
}

function repos::clone::all() {
  local workspace token
  workspace="${1}"
  token="${2}"

  util::print::blue "Fetching GitHub teams..."

  IFS=$'\n' read -r -d '' -a teams < <(
    teams::fetch paketo-buildpacks "${token}"
    teams::fetch paketo-community "${token}"

    printf '\0' # NULL-terminate the input
  )

  util::print::green "  Found ${#teams[@]} teams"
  util::print::break

  util::print::blue "Fetching GitHub repositories..."

  IFS=$'\n' read -r -d '' -a repos < <(
    for team in "${teams[@]}"; do
      local org name url
      org="$(jq -r .org <<< "${team}")"
      name="$(jq -r .slug <<< "${team}")"
      url="$(jq -r .repositories_url <<< "${team}")"

      teams::repos::fetch "${org}" "${name}" "${url}" "${token}"
    done | sort | uniq

    printf '\0' # NULL-terminate the input
  )

  util::print::green "  Found ${#repos[@]} repositories"
  util::print::break

  util::print::blue "Cloning repositories..."

  for repo in "${repos[@]}"; do
    local path url
    path="$(jq -r .full_name <<< "${repo}")"
    url="$(jq -r .ssh_url <<< "${repo}")"

    repo::fetch "${workspace}" "${url}" "${path}"
  done
}

function teams::fetch() {
  local org token
  org="${1}"
  token="${2}"

  util::print::yellow "  Fetching teams belonging to the ${org} GitHub organization..."

  curl "https://api.github.com/orgs/${org}/teams?per_page=100" \
    --fail-with-body \
    --show-error \
    --silent --location \
    --header "Accept: application/vnd.github.v3+json" \
    --header "Authorization: token ${token}" \
  | jq -r -c --arg org "${org}" '.[] | select(.slug | (contains("java") | not) and (contains("maintainers"))) | { org: $org, slug, repositories_url }'
}

function teams::repos::fetch() {
  local org team url token
  org="${1}"
  team="${2}"
  url="${3}"
  token="${4}"

  util::print::yellow "  Fetching repos belonging to the @${org}/${team} GitHub team..."

  curl "${url}" \
    --fail-with-body \
    --show-error \
    --silent --location \
    --header "Accept: application/vnd.github.v3+json" \
    --header "Authorization: token ${token}" \
  | jq -r -c '.[] | { full_name, ssh_url }'
}

function repo::fetch() {
  local workspace url path
  workspace="${1}"
  url="${2}"
  path="${workspace}/${3}"

  mkdir -p "$(dirname "${path}")"

  if [[ ! -d "${path}" ]]; then
    util::print::yellow "  Cloning ${url}..."

    git clone "${url}" "${path}" 2>&1 | util::print::indent | util::print::indent >&2
  else
    util::print::yellow "  Repository ${path} already exists, pulling..."

    git -C "${path}" checkout main 2>&1 | util::print::indent | util::print::indent >&2
    git -C "${path}" pull --rebase --autostash 2>&1 | util::print::indent | util::print::indent >&2
  fi
}

main "${@:-}"
