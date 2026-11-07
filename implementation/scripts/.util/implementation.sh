#!/usr/bin/env bash

set -eu
set -o pipefail

# Shared helpers for implementation repositories.
#
# A repository that packages a combined buildpack plus one buildpack per
# component opts in by adding a multi-implementation.json file at its root.
# Without that file these helpers return the standard single-buildpack
# defaults, so the workflows and scripts can follow one code path.

function util::implementation::config() {
  local root
  root="${1}"
  printf '%s/multi-implementation.json' "${root}"
}

function util::implementation::multi() {
  local root
  root="${1}"

  if [[ -f "$(util::implementation::config "${root}")" ]]; then
    printf 'true'
  else
    printf 'false'
  fi
}

function util::implementation::repo-name() {
  local root
  root="${1}"

  if [[ -n "${GITHUB_REPOSITORY:-}" ]]; then
    printf '%s' "${GITHUB_REPOSITORY#*/}"
  else
    basename "${root}"
  fi
}

function util::implementation::combined() {
  local root
  root="${1}"

  if [[ "$(util::implementation::multi "${root}")" == "true" ]]; then
    jq -r '.combined' "$(util::implementation::config "${root}")"
  else
    util::implementation::repo-name "${root}"
  fi
}

function util::implementation::buildpacks() {
  local root
  root="${1}"

  if [[ "$(util::implementation::multi "${root}")" == "true" ]]; then
    jq -c '[.buildpacks[].name]' "$(util::implementation::config "${root}")"
  else
    jq -nc --arg name "$(util::implementation::repo-name "${root}")" '[$name]'
  fi
}

function util::implementation::components() {
  local root combined
  root="${1}"

  if [[ "$(util::implementation::multi "${root}")" == "true" ]]; then
    combined="$(util::implementation::combined "${root}")"
    jq -c --arg combined "${combined}" \
      '[.buildpacks[].name | select(. != $combined)]' \
      "$(util::implementation::config "${root}")"
  else
    printf '[]'
  fi
}

function util::implementation::integration() {
  local root
  root="${1}"

  if [[ "$(util::implementation::multi "${root}")" == "true" ]]; then
    jq -r 'if has("integration") then .integration else true end' "$(util::implementation::config "${root}")"
  else
    printf 'true'
  fi
}

function util::implementation::image() {
  local root name image
  root="${1}"
  name="${2}"

  if [[ "$(util::implementation::multi "${root}")" == "true" ]]; then
    image="$(jq -r --arg name "${name}" '.buildpacks[] | select(.name == $name) | .image' "$(util::implementation::config "${root}")")"
  fi

  if [[ -z "${image:-}" || "${image}" == "null" ]]; then
    image="${name}"
  fi

  printf '%s' "${image}"
}

function util::implementation::asset-suffix() {
  local root name
  root="${1}"
  name="${2}"

  if [[ "$(util::implementation::multi "${root}")" == "true" ]]; then
    printf '%s' "-${name}"
  else
    printf ''
  fi
}
