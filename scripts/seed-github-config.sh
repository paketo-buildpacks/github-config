#!/usr/bin/env bash

set -eu
set -o pipefail

PROGDIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly PROGDIR
ROOT_DIR="$(cd "${PROGDIR}/.." && pwd)"
readonly ROOT_DIR
readonly SEED_BRANCH="automation/seed-github-config"
readonly PR_TITLE="Update github-config workflow to authenticate push"

function usage() {
  cat <<-USAGE
seed-github-config.sh --workspace <path> [OPTIONS]

One-time seed to unblock repositories whose update-github-config workflow can
no longer push (because it relied on persisted git credentials). Copies the
current workflow file(s) from this github-config checkout into each matching
repository, pushes a branch, and opens a pull request. Once merged, the
repository can self-update again.

Repositories are discovered by scanning "<workspace>/paketo-buildpacks/*" for a
.github/workflows/update-github-config.yml referencing the given config type.

OPTIONS
  --workspace <path>        directory containing cloned paketo-buildpacks repos
  --config-type <name>      config directory to seed from (default: language-family)
  --file <path>             file to seed, relative to the config dir (repeatable;
                            default: .github/workflows/update-github-config.yml)
  --dry-run                 print what would be done without changing anything
  --help  -h                prints the command usage
USAGE
}

function main() {
  local workspace config_type dry_run
  workspace=""
  config_type="language-family"
  dry_run=false
  local files=()

  while [[ "${#}" != 0 ]]; do
    case "${1}" in
      --workspace)
        workspace="${2}"
        shift 2
        ;;

      --config-type)
        config_type="${2}"
        shift 2
        ;;

      --file)
        files+=("${2}")
        shift 2
        ;;

      --dry-run)
        dry_run=true
        shift 1
        ;;

      --help|-h)
        shift 1
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

  if [[ -z "${workspace}" ]]; then
    usage
    exit 1
  fi

  if [[ "${#files[@]}" == 0 ]]; then
    files=(".github/workflows/update-github-config.yml")
  fi

  seed "${workspace}" "${config_type}" "${dry_run}" "${files[@]}"
}

function seed() {
  local workspace config_type dry_run
  workspace="${1}"
  config_type="${2}"
  dry_run="${3}"
  shift 3
  local files=("${@}")

  local base="${workspace}/paketo-buildpacks"
  if [[ ! -d "${base}" ]]; then
    echo "error: \"${base}\" does not exist; clone repositories first" >&2
    exit 1
  fi

  local seeded=()
  local skipped=()
  local failed=()

  local repo_path name target default_branch changed
  for repo_path in "${base}"/*; do
    [[ -d "${repo_path}/.git" ]] || continue
    name="$(basename "${repo_path}")"
    target="${repo_path}/.github/workflows/update-github-config.yml"

    if [[ ! -f "${target}" ]] || ! grep -q "github-config/${config_type}" "${target}"; then
      continue
    fi

    changed=false
    for file in "${files[@]}"; do
      if ! cmp -s "${ROOT_DIR}/${config_type}/${file}" "${repo_path}/${file}"; then
        changed=true
      fi
    done

    if [[ "${changed}" == false ]]; then
      skipped+=("${name}")
      continue
    fi

    if [[ "${dry_run}" == true ]]; then
      echo "would seed ${name}"
      seeded+=("${name} (dry-run)")
      continue
    fi

    default_branch="$(git -C "${repo_path}" symbolic-ref --short refs/remotes/origin/HEAD 2>/dev/null | sed 's#^origin/##' || true)"
    if [[ -z "${default_branch}" ]]; then
      default_branch="main"
    fi

    if (
      git -C "${repo_path}" fetch --quiet origin
      git -C "${repo_path}" checkout --quiet -B "${SEED_BRANCH}" "origin/${default_branch}"
      for file in "${files[@]}"; do
        mkdir -p "${repo_path}/$(dirname "${file}")"
        cp "${ROOT_DIR}/${config_type}/${file}" "${repo_path}/${file}"
        git -C "${repo_path}" add "${file}"
      done
      git -C "${repo_path}" commit --quiet -m "${PR_TITLE}"
      git -C "${repo_path}" push --quiet --force-with-lease -u origin "${SEED_BRANCH}"
      gh pr create \
        --repo "paketo-buildpacks/${name}" \
        --base "${default_branch}" \
        --head "${SEED_BRANCH}" \
        --title "${PR_TITLE}" \
        --body "One-time seed so this repository can update itself again after the github-config push-branch hardening. Once merged, the scheduled update-github-config workflow will sync the remaining files." \
      || echo "  (a pull request for ${name} may already exist)"
    ); then
      seeded+=("${name}")
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

if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
  main "${@:-}"
fi
