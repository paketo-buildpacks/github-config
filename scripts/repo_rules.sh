#!/usr/bin/env bash

set -eu
set -o pipefail

readonly PROGDIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# shellcheck source=SCRIPTDIR/.util/print.sh
source "${PROGDIR}/.util/print.sh"

function main() {
  local repo token branch verbose
  while [[ "${#}" != 0 ]]; do
    case "${1}" in
      --repo)
        repo="${2}"
        shift 2
        ;;

      --token)
        token="${2}"
        shift 2
        ;;

      --branch)
        branch="${2}"
        shift 2
        ;;

      --verbose)
        verbose=true
        shift 1
        ;;

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

  if [[ -z "${repo:-}" ]]; then
    usage
    echo
    util::print::error "--repo is a required flag"
  fi

  if [[ -z "${token:-}" ]]; then
    usage
    echo
    util::print::error "--token is a required flag"
  fi

  if [[ -z "${branch:-}" ]]; then
    branch="main"
  fi

  if [[ ! "${repo}" =~ [a-z-]+/[a-z-]+ ]]; then
    util::print::error "--repo argument must match <org>/<name> format"
  fi

  if rules "${token}" "${repo}" "${branch}" "${verbose}"; then
    util::print::success "Valid"
  else
    util::print::error "Invalid"
  fi
}

function usage() {
  cat <<-USAGE
repo_rules.sh --repo <repo> --token <token> [OPTIONS]

Validates branch protection rules for a GitHub repository.

OPTIONS
  --branch <branch>  branch to check for protection rules (default: main)
  --help  -h         prints the command usage
  --repo <repo>      name of the GitHub repository to check in the form <org>/<name>
  --token <token>    GitHub token used to check the repository
  --verbose          Print the JSON returned from the API
USAGE
}

function rules() {
  local token repo branch verbose json
  token="${1}"
  repo="${2}"
  branch="${3}"
  verbose="${4}"

  json="$(
    curl "https://api.github.com/repos/${repo}/branches/${branch}/protection" \
      --silent \
      --request GET \
      --header "Accept: application/vnd.github.luke-cage-preview+json" \
      --header "Authorization: token ${token}"
  )"

  if [[ -n "${verbose}" ]]; then
    echo "${json}" | jq -r .
  fi

  local valid
  valid=0

  if ! rules::present "$(jq -r .url <<< "${json}")" "${branch}"; then
    valid=1
  fi

  if ! rules::reviews::count "$(jq .required_pull_request_reviews.required_approving_review_count <<< "${json}")"; then
    valid=1
  fi

  if ! rules::reviews::stale::dismiss "$(jq .required_pull_request_reviews.dismiss_stale_reviews <<< "${json}")"; then
    valid=1
  fi

  if ! rules::reviews::codeowner "$(jq .required_pull_request_reviews.require_code_owner_reviews <<< "${json}")"; then
    valid=1
  fi

  if ! rules::checks::strict "$(jq .required_status_checks.strict <<< "${json}")"; then
    valid=1
  fi

  if ! rules::checks::integration "$(jq '.required_status_checks.contexts | index("Integration Tests")' <<< "${json}")"; then
    valid=1
  fi

  if ! rules::checks::labels "$(jq '.required_status_checks.contexts | index("Ensure Minimal Semver Labels")' <<< "${json}")"; then
    valid=1
  fi

  if ! rules::history::linear "$(jq .required_linear_history.enabled <<< "${json}")"; then
    valid=1
  fi

  if ! rules::administrators::include "$(jq .enforce_admins.enabled <<< "${json}")"; then
    valid=1
  fi

  if ! rules::push::force::deny "$(jq .allow_force_pushes.enabled <<< "${json}")" "${branch}"; then
    valid=1
  fi

  if ! rules::branch::delete::deny "$(jq .allow_deletions.enabled <<< "${json}")"; then
    valid=1
  fi

  util::print::break

  return ${valid}
}

function rules::present() {
  local url branch
  url="${1}"
  branch="${2}"

  if [[ -z "${url}" || "${url}" == "null" ]]; then
    util::print::error "No branch protection rules defined for ${branch} (or you do not have permission)"
  fi
}

function rules::reviews::count() {
  local pr_review_count
  pr_review_count="${1}"

  if [[ "${pr_review_count}" != "1" ]]; then
    util::print::yellow 'Merging: Required approving reviews is not 1'
    return 1
  fi
}

function rules::reviews::stale::dismiss() {
  local dismiss
  dismiss="${1}"

  if [[ "${dismiss}" != "true" ]]; then
    util::print::yellow 'Merging: Dismiss stale pull request approvals when new commits are pushed - not enabled'
    return 1
  fi
}

function rules::reviews::codeowner() {
  local codeowner_review
  codeowner_review="${1}"

  if [[ "${codeowner_review}" != "true" ]]; then
    util::print::yellow 'Merging: Require review from a codeowner - not enabled'
    return 1
  fi
}

function rules::checks::strict() {
  local status_checks
  status_checks="${1}"

  if [[ "${status_checks}" != "true" ]]; then
    util::print::yellow 'Merging: Require status checks to pass before merge - not enabled'
    return 1
  fi
}

function rules::checks::integration() {
  local status_checks_int
  status_checks_int="${1}"

  if [[ -z "${status_checks_int}" || "${status_checks_int}" == "null" ]]; then
    util::print::yellow 'Merging: Required status checks do not contain Integration Tests'
    return 1
  fi
}

function rules::checks::labels() {
  local status_checks_labels
  status_checks_labels="${1}"

  if [[ -z "${status_checks_labels}" || "${status_checks_labels}" == "null" ]]; then
    util::print::yellow 'Merging: Required status checks do not contain "Ensure Minimal Semver Labels"'
    return 1
  fi
}

function rules::history::linear() {
  local linear_history
  linear_history="${1}"

  if [[ "${linear_history}" != "true" ]]; then
    util::print::yellow 'Require linear history - not enabled'
    return 1
  fi
}

function rules::administrators::include() {
  local enforce_admins
  enforce_admins="${1}"

  if [[ "${enforce_admins}" != "true" ]]; then
    util::print::yellow 'Enforce all restrictions for admins - not enabled'
    return 1
  fi
}

function rules::push::force::deny() {
  local force_pushes branch
  force_pushes="${1}"
  branch="${2}"

  if [[ "$force_pushes" != "false" ]]; then
    util::print::yellow "Allow force pushes to ${branch} - enabled"
    return 1
  fi
}

function rules::branch::delete::deny() {
  local deletions
  deletions="${1}"

  if [[ "$deletions" != "false" ]]; then
    util::print::yellow "Allow users to delete $branch - enabled"
    return 1
  fi
}

main "${@:-}"
