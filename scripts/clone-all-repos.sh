#!/bin/bash
set -eu

readonly PROGDIR="$(cd "$(dirname "${0}")" && pwd)"
readonly WORKSPACE="${HOME}/workspace"

# shellcheck source=SCRIPTDIR/.util/print.sh
source "${PROGDIR}/.util/print.sh"

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

  if [ -z "${GIT_TOKEN}" ]; then
    util::print::error "Must set GIT_TOKEN"
  fi

  clone_all_repos

  util::print::success "All repos cloned. Look around in ${WORKSPACE}"
}

function usage() {
  cat <<-USAGE
clone-all-repos.sh [OPTIONS]

Clones relevant Paketo Buildpacks and Paketo Community repos into
~/workspace/<org>/<repo>. Requires a \$GIT_TOKEN for Github API requests.

OPTIONS
  --help       -h  prints the command usage
USAGE
}

function clone_all_repos() {
  while read -r line; do
    clone_team_repos ${line}
  done < <( get_org_teams paketo-buildpacks | sort)

  while read -r line; do
    clone_team_repos ${line}
  done < <( get_org_teams paketo-community | sort)
}

function get_org_teams(){
  local org

  org="${1}"

  curl --silent \
  -H "Accept: application/vnd.github.v3+json" \
  -H "Authorization: token ${GIT_TOKEN}" \
  "https://api.github.com/orgs/${org}/teams?per_page=100" \
  | jq -r '.[] | select(.slug | (contains("java") | not) and (contains("maintainers"))) | "\(.slug) \(.repositories_url)"'
}

function clone_team_repos() {
  local team_name repositories_url

  team_name="${1}"
  repositories_url="${2}"

  repos="$(curl --silent \
  -H "Accept: application/vnd.github.v3+json" \
  -H "Authorization: token ${GIT_TOKEN}" \
  "${repositories_url}" | jq -r '.[] | "\(.ssh_url) \(.full_name)"' )"

  util::print::info "Cloning ${team_name} repos..."

  while read -r line; do
    clone_or_pull ${line}
  done < <( echo "${repos}" | sort)
}

function clone_or_pull() {
  local ssh_url repo_path
  ssh_url="${1}"
  repo_path="${2}"

  path="${WORKSPACE}/${repo_path}"

  mkdir -p "$(dirname "${path}")"

  if [[ ! -d "${path}" ]]; then
    echo "Cloning ${repo_path}"
    git clone "${ssh_url}" "${path}"
  else
    echo "${repo_path} already cloned, updating"

    git -C "${path}" checkout main
    git -C "${path}" pull --rebase --autostash
  fi
}

main "${@:-}"
