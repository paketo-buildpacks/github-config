#!/usr/bin/env bash

set -eu -o pipefail
shopt -s inherit_errexit

readonly ROOTDIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )/.." && pwd )"

# shellcheck source=SCRIPTDIR/.util/print.sh
source "${ROOTDIR}/scripts/.util/print.sh"

if [[ $BASH_VERSINFO -lt 4 ]]; then
  util::print::error "Before running this script please update Bash to v4 or higher (e.g. on OSX: \$ brew install bash)"
fi

if [[ ! $(which gh) ]]; then
  util::print::error "Before running this script please first install gh (e.g. on OSX: \$ brew install gh)"
fi

function main() {
  local orgs teams default_orgs

  default_orgs=(
    "paketo-buildpacks"
    "paketo-community"
  )

  orgs=()
  teams=()

  while [[ "${#}" != 0 ]]; do
    case "${1}" in
      "--org")
        orgs+=("${2}")
        shift 2
        ;;

      "--team")
        teams+=("${2}")
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

  # ${#arr[@]} calculates array length
  if [[ ${#teams[@]} -eq 0 ]]; then
    util::print::error "must provide at least one team regex with --team"
  fi

  if [[ ${#orgs[@]} -eq 0 ]]; then
    orgs=(${default_orgs[@]})
  fi

  gh auth status

  message="Getting teams in orgs [ ${orgs[@]} ]"
  util::print::info "${message}"

  filtered_teams_json='[]'
  for org in "${orgs[@]}"; do
    # ${arr[@]} retrieves all array elements.
    # No quotes around the expansion otherwise it becomes a single string
    org_teams=$(find_teams "${org}" ${teams[@]})

    filtered_teams_json=$(
      echo "${filtered_teams_json}" \
        | jq \
          --compact-output \
          --argjson org_teams "${org_teams}" \
          '. += $org_teams'
    )
  done

  team_count=$(echo "${filtered_teams_json}" | jq length)
  if [[ "${team_count}" -eq 0 ]]; then
    message="No 'maintainer' teams found in orgs: [ ${orgs[@]} ] for teams: [ ${teams[@]} ]"
    util::print::error "${message}"
  fi

  # jq @sh prints items in a way that a shell can process them (with single quotes around them) e.g.:
  # 'some-org/some-team'
  # We can remove the single quotes (via sed) because the org and team slugs don't have spaces.
  # We haven't found a better way to print multiple strings on a single line separated by a space
  teams_printable=$(echo "${filtered_teams_json}" | jq -r 'map("\(.org)/\(.slug)")|@sh' | sed "s/'//g")
  message="Getting repos for teams: [ ${teams_printable[@]} ]"
  util::print::info "${message}"

  filtered_repos_json='[]'
  while read -r team_json; do
    found_repos=$(find_repos "${team_json}")

    filtered_repos_json=$(
      echo "${filtered_repos_json}" \
        | jq \
          --compact-output \
          --argjson found_repos "${found_repos}" \
          '. += $found_repos'
    )
  done < <(echo "${filtered_teams_json}" | jq --compact-output '.[]')

  repo_count=$(echo "${filtered_repos_json}" | jq length)
  if [[ "${repo_count}" -eq 0 ]]; then
    util::print::error "No repos found"
  fi

  util::print::break
  while read -r repo_json; do
    publish_draft_release "${repo_json}"
  done < <(echo "${filtered_repos_json}" | jq --compact-output '.[]')
}

function find_teams(){
  local org
  org="${1}"
  shift 1
  teams=("${@}") # assign all remaining (varadic) arguments to the "teams" array

  # Use jq --slurp 'add' because pagination can return multiple arrays and we want to treat them as a single array
  teams_json=$(
    gh api \
      --paginate \
      "orgs/${org}/teams" \
        | jq \
            --compact-output \
            --slurp \
            --arg org "${org}" \
            'add | map({"org":$org, "slug":.slug,"name":.name})|map(select(.name|test("[Mm]aintainer")))'
  )

  selected_teams_json='[]'
  for provided_team_name in ${teams[@]}; do
    selected_teams_json=$(
      echo "${selected_teams_json}" \
        | jq \
            --arg provided_team_name "${provided_team_name}" \
            --argjson teamsjson "${teams_json}" \
            '. += ($teamsjson | map(select(.name | ascii_downcase |test("\($provided_team_name)"))))'
      )
  done

  echo "${selected_teams_json}" | jq --compact-output '.'
}

function find_repos() {
  local teams_json
  teams_json="${1}"

  org=$(echo "${teams_json}" | jq --raw-output '.org')
  team_slug=$(echo "${teams_json}" | jq --raw-output '.slug')

  # Use jq --slurp 'add' because pagination can return multiple arrays and we want to treat them as a single array
  gh api \
    --paginate \
    "orgs/${org}/teams/${team_slug}/repos" \
      | jq \
          --compact-output \
          --slurp \
          'add | map(select(.role_name=="admin")) | map({"full_name":.full_name})'
}

function publish_draft_release() {
  local repo_json
  repo_json="${1}"

  full_name=$(echo "${repo_json}" | jq --raw-output '.full_name')
  util::print::info "Getting draft releases for ${full_name}"

  # Use jq --slurp 'add' because pagination can return multiple arrays and we want to treat them as a single array
  draft_releases_json=$(
    gh api \
        --paginate \
        "/repos/${full_name}/releases" \
          | jq \
            --compact-output \
            --slurp \
            'add | map(select(.draft==true))|map({"id":.id,"tag_name":.tag_name,"target_commitish":.target_commitish})'
  )

  draft_release_count=$(echo "${draft_releases_json}" | jq length)
  if [[ "${draft_release_count}" -eq 0 ]]; then
    util::print::info "↳ None found"
    util::print::break
    return
  fi

  if [[ "${draft_release_count}" -gt 1 ]]; then
    util::print::error "multiple draft releases found for ${full_name}"
  fi

  latest_release_json=$(gh api "repos/${full_name}/releases/latest" | jq --compact-output '.')
  latest_release_tag=$(echo "${latest_release_json}" | jq --raw-output '.tag_name')

  draft_release_id=$(echo "${draft_releases_json}" | jq '.[0].id')
  draft_release_tag=$(echo "${draft_releases_json}" | jq --raw-output '.[0].tag_name')
  draft_release_commitish=$(echo "${draft_releases_json}" | jq --raw-output '.[0].target_commitish')

  util::print::title "Draft release found for ${full_name} ($draft_release_tag)"
  util::print::info "Commits since previous published release (${latest_release_tag}):"

  # Use jq --slurp 'add' because pagination can return multiple arrays and we want to treat them as a single array
  # Use .[]? to return early (without error) from jq if there are no commits.
  gh api \
    --paginate \
    "/repos/${full_name}/compare/${latest_release_tag}...${draft_release_commitish}" \
      | jq \
        --slurp \
        'add | .commits | .[]? | .commit | {"author":.author.name,"message":.message}'

  # Wait for user input
  read -p "Publish release for ${full_name} (y/n)? " resp < /dev/tty

  if [[ "${resp}" == "y" || "${resp}" == "yes" ]];then
    url=$(
      gh api \
        --method PATCH \
        -F draft=false \
        "/repos/${full_name}/releases/${draft_release_id}" \
          | jq --raw-output .html_url
    )
    util::print::green "release published at: ${url}"
  else
    util::print::yellow "not publishing release for ${full_name}"
  fi

  util::print::break
}

main "${@:-}"
