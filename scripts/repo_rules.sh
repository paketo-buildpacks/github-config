#!/usr/bin/env bash
# Script to validate repo rules for paketo cnbs
# usage e.g.
# GITHUB_TOKEN=$(cat /path/to/github_token) repo_rules.sh paketo-buildpacks/php-dist
# Pass second argument --verbose to print the api output json
# You can get a personal token from github.com/settings. Token just requires repo perm

set -e
set -u
set -o pipefail

if [  $# -lt 1 ] || [  -z "${GITHUB_TOKEN:-}" ]; then
    echo "usage: GITHUB_TOKEN=<token> $0 <org>/<repo> [--verbose]"; exit 1;
fi

if ! [[ "$1" =~ [a-z-]+/[a-z-]+ ]]; then
    echo Provide a valid argument of syntax "<org>/<repo>"
    exit 1
fi

user=${1%%/*}
repo=${1##*/}
green="\\0033[0;32m"
red="\\0033[0;31m"
reset="\\0033[0;39m"
ret=0

bad() {
    echo -e "- $red""$1""$reset"
    ret=1
}

# Branch Protection Rules
branch_protection() {
    branch=main
    json=$(curl -s -X GET https://api.github.com/repos/"$user"/"$repo"/branches/"$branch"/protection \
    -H 'Accept: application/vnd.github.luke-cage-preview+json' \
    -H "Authorization: token ${GITHUB_TOKEN}")

    if [ "$#" -eq 2 ] && [ "$2" == "--verbose" ]; then
	echo "$json"
    fi

    apiurl=$(jq .url <<< "$json")
    if [ -z "$apiurl" ] || [ "$apiurl" == "null" ]; then
	bad "No branch protection rules defined for $branch (or you do not have permission)"
	exit 1
    fi

    status_checks=$(jq .required_status_checks.strict <<< "$json")
    if [ "$status_checks" != "true" ]; then
	bad 'Merging: Require status checks to pass before merge - not enabled'
    fi

    status_checks_int=$(jq '.required_status_checks.contexts | index("Integration Tests")' <<< "$json")
    if [ -z "$status_checks_int" ] || [ "$status_checks_int" == "null" ]; then
	bad 'Merging: Required status checks do not contain Integration Tests'
    fi

    pr_review_count=$(jq .required_pull_request_reviews.required_approving_review_count <<< "$json")
    if [ "$pr_review_count" != "1" ]; then
	bad 'Merging: Required approving reviews is not 1'
    fi

    codeowner_review=$(jq .required_pull_request_reviews.require_code_owner_reviews <<< "$json")
    if [ "$codeowner_review" != "true" ]; then
	bad 'Merging: Require review from a codeowner - not enabled'
    fi

    linear_history=$(jq .required_linear_history.enabled <<< "$json")
    if [ "$linear_history" != "true" ]; then
	bad 'Require linear history - not enabled'
    fi

    enforce_admins=$(jq .enforce_admins.enabled <<< "$json")
    if [ "$enforce_admins" != "true" ]; then
	bad 'Enforce all restrictions for admins - not enabled'
    fi

    force_pushes=$(jq .allow_force_pushes.enabled <<< "$json")
    if [ "$force_pushes" != "false" ]; then
	bad "Allow force pushes to $branch - enabled"
    fi

    deletions=$(jq .allow_deletions.enabled <<< "$json")
    if [ "$deletions" != "false" ]; then
	bad "Allow users to delete $branch - enabled"
    fi
}

branch_protection "$@"

[ "$ret" -eq 0 ] && echo -e "$green"Valid"$reset"
exit $ret
