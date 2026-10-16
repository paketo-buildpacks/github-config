#!/usr/bin/env bash

set -eu
set -o pipefail

PROGDIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly PROGDIR
ROOT_DIR="$(cd "${PROGDIR}/.." && pwd)"
readonly ROOT_DIR
readonly ORG="paketo-buildpacks"
readonly SEED_BRANCH="automation/seed-github-config"
readonly PR_TITLE="Update github-config to authenticate push"

function usage() {
  cat <<-USAGE
seed-github-config.sh [OPTIONS]

Seeds repositories that can no longer update themselves: their
update-github-config workflow relied on persisted git credentials, which the
shared push-branch action no longer uses, so its push fails and the repo is
stuck.

This clones the target repositories into a throwaway temporary directory,
applies the current github-config config to each, pushes a branch, and opens a
pull request. Nothing outside the temporary directory is touched, so it is safe
to run against a machine that already has clones lying around.

OPTIONS
  --config-type <name>   config directory to seed from (default: language-family)
  --repo <name>          repository to seed (repeatable). When omitted, every
                         active ${ORG} repository that references
                         "github-config/<config-type>" is discovered and seeded.
  --dry-run              print what would be done without changing anything
  --keep                 keep the temporary working directory for inspection
  --help  -h             prints the command usage
USAGE
}

function main() {
  local config_type dry_run keep
  config_type="language-family"
  dry_run=false
  keep=false
  local repos=()

  while [[ "${#}" != 0 ]]; do
    case "${1}" in
      --config-type)
        config_type="${2}"
        shift 2
        ;;

      --repo)
        repos+=("${2#"${ORG}/"}")
        shift 2
        ;;

      --dry-run)
        dry_run=true
        shift 1
        ;;

      --keep)
        keep=true
        shift 1
        ;;

      --help|-h)
        usage
        exit 0
        ;;

      "")
        shift 1
        ;;

      *)
        echo "unknown argument \"${1}\"" >&2
        usage
        exit 1
    esac
  done

  if [[ ! -d "${ROOT_DIR}/${config_type}" ]]; then
    echo "error: config type \"${config_type}\" not found at ${ROOT_DIR}/${config_type}" >&2
    exit 1
  fi

  if [[ "${#repos[@]}" == 0 ]]; then
    local name
    while IFS= read -r name; do
      [[ -n "${name}" ]] && repos+=("${name}")
    done < <(discover "${config_type}")
  fi

  if [[ "${#repos[@]}" == 0 ]]; then
    echo "no repositories found to seed"
    return 0
  fi

  local workdir
  workdir="$(mktemp -d "${TMPDIR:-/tmp}/seed-github-config.XXXXXX")"
  if [[ "${keep}" == true ]]; then
    echo "working directory: ${workdir}"
  else
    trap "rm -rf '${workdir}'" EXIT
  fi

  seed "${workdir}" "${config_type}" "${dry_run}" "${repos[@]}"
}

# discover prints the names of active repositories that use the given config.
function discover() {
  local config_type name content
  config_type="${1}"
  while IFS= read -r name; do
    content="$(gh api \
      -H "Accept: application/vnd.github.raw" \
      "repos/${ORG}/${name}/contents/.github/workflows/update-github-config.yml" \
      2>/dev/null || true)"
    if [[ -n "${content}" ]] && grep -q "github-config/${config_type}" <<<"${content}"; then
      echo "${name}"
    fi
  done < <(gh repo list "${ORG}" --limit 400 --json name,isArchived,isFork \
    --jq '.[] | select(.isArchived == false and .isFork == false) | .name')
}

function seed() {
  local workdir config_type dry_run
  workdir="${1}"
  config_type="${2}"
  dry_run="${3}"
  shift 3
  local repos=("${@}")

  local seeded=()
  local skipped=()
  local failed=()
  local name rc

  for name in "${repos[@]}"; do
    rc=0
    seed_repo "${workdir}" "${config_type}" "${dry_run}" "${name}" || rc="${?}"

    if [[ "${rc}" == 0 ]]; then
      seeded+=("${name}")
    elif [[ "${rc}" == 3 ]]; then
      skipped+=("${name}")
    else
      failed+=("${name}")
    fi
  done

  echo
  echo "seeded:  ${seeded[*]:-none}"
  echo "skipped: ${skipped[*]:-none}"
  echo "failed:  ${failed[*]:-none}"

  if [[ "${#failed[@]}" != 0 ]]; then
    exit 1
  fi
}

# seed_repo clones and seeds one repository. Returns 0 when a pull request was
# opened, 3 when the repository was already up to date, and 1 on failure.
function seed_repo() {
  local workdir config_type dry_run name
  workdir="${1}"
  config_type="${2}"
  dry_run="${3}"
  name="${4}"
  local repo_path="${workdir}/${name}"

  echo "=== ${name} ==="

  if ! gh repo clone "${ORG}/${name}" "${repo_path}" -- --quiet; then
    echo "  failed to clone ${ORG}/${name}" >&2
    return 1
  fi

  local default_branch
  default_branch="$(git -C "${repo_path}" symbolic-ref --short refs/remotes/origin/HEAD 2>/dev/null | sed 's#^origin/##' || true)"
  if [[ -z "${default_branch}" ]]; then
    default_branch="main"
  fi

  git -C "${repo_path}" checkout --quiet -B "${SEED_BRANCH}" "origin/${default_branch}"

  sync_config "${repo_path}" "${config_type}"

  if [[ -z "$(git -C "${repo_path}" status --porcelain)" ]]; then
    echo "  already up to date"
    return 3
  fi

  if [[ "${dry_run}" == true ]]; then
    echo "  would seed:"
    git -C "${repo_path}" status --short | sed 's/^/    /'
    return 0
  fi

  git -C "${repo_path}" add -A
  git -C "${repo_path}" commit --quiet -m "${PR_TITLE}"
  git -C "${repo_path}" push --quiet --force-with-lease -u origin "${SEED_BRANCH}"

  if ! gh pr create \
    --repo "${ORG}/${name}" \
    --base "${default_branch}" \
    --head "${SEED_BRANCH}" \
    --title "${PR_TITLE}" \
    --body "One-time seed so this repository can update itself again after the github-config push-branch hardening. Applies the current \`${config_type}\` config."; then
    echo "  (a pull request for ${name} may already exist)"
  fi

  echo "  seeded"
  return 0
}

# sync_config mirrors actions/sync: rsync each top-level directory from the
# config into the repository, honoring the repository's .syncignore and running
# its .sync-post-hook if present.
function sync_config() {
  local repo_path config_type
  repo_path="${1}"
  config_type="${2}"
  local config="${ROOT_DIR}/${config_type}"

  local dir src dest
  while IFS= read -r dir; do
    [[ -n "${dir}" ]] || continue

    src="${config}/${dir}/"
    dest="${repo_path}/${dir}/"
    mkdir -p "${dest}"

    local args=(--recursive --checksum "${src}" "${dest}" --delete)
    if [[ -f "${dest}/.syncignore" ]]; then
      args+=(--exclude=".syncignore" "--exclude-from=${dest}/.syncignore")
    fi

    rsync "${args[@]}"

    if [[ -f "${dest}/.sync-post-hook" ]]; then
      (cd "${repo_path}" && bash "${dest}/.sync-post-hook")
    fi
  done < <(cd "${config}" && find . -mindepth 1 -maxdepth 1 -type d -print | sed 's#^\./##')
}

if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
  main "${@:-}"
fi
