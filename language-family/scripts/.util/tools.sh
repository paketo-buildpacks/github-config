#!/usr/bin/env bash

set -eu
set -o pipefail

# shellcheck source=SCRIPTDIR/print.sh
source "$(dirname "${BASH_SOURCE[0]}")/print.sh"

function util::tools::path::export() {
  local dir
  dir="${1}"

  if ! echo "${PATH}" | grep -q "${dir}"; then
    PATH="${dir}:$PATH"
    export PATH
  fi
}

function util::tools::jam::install () {
  local dir
  while [[ "${#}" != 0 ]]; do
    case "${1}" in
      --directory)
        dir="${2}"
        shift 2
        ;;

      *)
        util::print::error "unknown argument \"${1}\""
    esac
  done

  local os
  case "$(uname)" in
    "Darwin")
      os="darwin"
      ;;

    "Linux")
      os="linux"
      ;;

    *)
      echo "Unknown OS \"$(uname)\""
      exit 1
  esac

  mkdir -p "${dir}"
  util::tools::path::export "${dir}"

  if [[ ! -f "${dir}/jam" ]]; then
    local version
    version="v0.3.1"

    util::print::title "Installing jam ${version}"
    curl "https://github.com/paketo-buildpacks/packit/releases/download/${version}/jam-${os}" \
      --silent \
      --location \
      --output "${dir}/jam"
    chmod +x "${dir}/jam"
  fi
}

function util::tools::pack::install() {
  local dir
  while [[ "${#}" != 0 ]]; do
    case "${1}" in
      --directory)
        dir="${2}"
        shift 2
        ;;

      *)
        util::print::error "unknown argument \"${1}\""
    esac
  done

  mkdir -p "${dir}"
  util::tools::path::export "${dir}"

  local os
  case "$(uname)" in
    "Darwin")
      os="macos"
      ;;

    "Linux")
      os="linux"
      ;;

    *)
      echo "Unknown OS \"$(uname)\""
      exit 1
  esac

  if [[ ! -f "${dir}/pack" ]]; then
    local version
    version="v0.14.2"

    util::print::title "Installing pack ${version}"
    curl "https://github.com/buildpacks/pack/releases/download/${version}/pack-${version}-${os}.tgz" \
      --silent \
      --location \
      --output /tmp/pack.tgz
    tar xzf /tmp/pack.tgz -C "${dir}"
    chmod +x "${dir}/pack"
    rm /tmp/pack.tgz
  fi
}

function util::tools::packager::install () {
    local dir
    while [[ "${#}" != 0 ]]; do
      case "${1}" in
        --directory)
          dir="${2}"
          shift 2
          ;;

        *)
          util::print::error "unknown argument \"${1}\""
      esac
    done

    mkdir -p "${dir}"
    util::tools::path::export "${dir}"

    if [[ ! -f "${dir}/packager" ]]; then
        util::print::title "Installing packager"
        GOBIN="${dir}" go get github.com/cloudfoundry/libcfbuildpack/packager
    fi
}

function util::tools::image::reference() {
  local image digest
  image="${1}"
  digest="$(
    docker inspect --format='{{index .RepoDigests 0}}' "${image}" \
      | cut -d '@' -f 2
  )"
  echo "${image}@${digest}"
}

function util::tools::tests::checkfocus() {
  testout="${1}"
  if grep -q 'Focused: [1-9]' "${testout}"; then
    echo "Detected Focused Test(s) - setting exit code to 197"
    rm "${testout}"
    util::print::success "** GO Test Succeeded **" 197
  fi
  rm "${testout}"
}

function util::tools::tests::dump() {
  local output_file builder run lifecycle json
  output_file="${1}"
  builder="$(util::tools::image::reference "${2}")"
  run="$(util::tools::image::reference "${3}")"
  lifecycle="$(util::tools::image::reference "${4}")"

  json="$(
    jq -n \
      --arg builder "${builder}" \
      --arg run "${run}" \
      --arg lifecycle "${lifecycle}" \
      --arg pack_version "$(pack --version)" \
      '{builder: $builder, run: $run, lifecycle: $lifecycle, pack: $pack_version}' \
  )"
  echo "${json}" > "${output_file}"
}
