#!/usr/bin/env bash

set -eu
set -o pipefail

readonly ROOT_DIR="$(cd "$(dirname "${0}")/.." && pwd)"
readonly BIN_DIR="${ROOT_DIR}/.bin"
readonly BUILD_DIR="${ROOT_DIR}/build"
readonly CONFIG="${ROOT_DIR}/multi-implementation.json"

# shellcheck source=SCRIPTDIR/.util/tools.sh
source "${ROOT_DIR}/scripts/.util/tools.sh"

# shellcheck source=SCRIPTDIR/.util/print.sh
source "${ROOT_DIR}/scripts/.util/print.sh"

function main {
  local version output token
  token=""

  while [[ "${#}" != 0 ]]; do
    case "${1}" in
      --version|-v)
        version="${2}"
        shift 2
        ;;

      --output|-o)
        output="${2}"
        shift 2
        ;;

      --token|-t)
        token="${2}"
        shift 2
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

  if [[ -z "${version:-}" ]]; then
    usage
    echo
    util::print::error "--version is required"
  fi

  if [[ -z "${output:-}" ]]; then
    output="${BUILD_DIR}/buildpackage.cnb"
  fi

  if [[ ! -f "${CONFIG}" ]]; then
    util::print::error "cannot package: ${CONFIG} does not exist"
  fi

  repo::prepare

  tools::install "${token}"

  buildpack_type=buildpack
  if [ -f "${ROOT_DIR}/extension.toml" ]; then
    buildpack_type=extension
  fi

  buildpackage::create::individual "${version}" "${buildpack_type}"
  buildpackage::create::combined "${version}" "${buildpack_type}"
}

function usage() {
  cat <<-USAGE
package.sh --version <version> [OPTIONS]

Packages the combined buildpack and one buildpack per component into
buildpackage .cnb files.

The components and the libpak-tools command are read from
${ROOT_DIR}/multi-implementation.json.

OPTIONS
  --help               -h            prints the command usage
  --version <version>  -v <version>  specifies the version number to use when packaging a buildpack or an extension
  --output <output>    -o <output>   location to output the packaged buildpackage or extension artifact (unused)
  --token <token>                    Token used to download assets from GitHub (e.g. jam, pack, etc) (optional)
USAGE
}

function repo::prepare() {
  util::print::title "Preparing repo..."

  rm -rf "${BUILD_DIR}"

  mkdir -p "${BIN_DIR}"
  mkdir -p "${BUILD_DIR}"

  export PATH="${BIN_DIR}:${PATH}"
}

function tools::install() {
  local token
  token="${1}"

  util::tools::pack::install \
    --directory "${BIN_DIR}" \
    --token "${token}"

  if [[ -f "${ROOT_DIR}/.libbuildpack" ]]; then
    util::tools::packager::install \
      --directory "${BIN_DIR}"
  else
    util::tools::libpak-tools::install \
      --directory "${BIN_DIR}" \
      --token "${token}"
  fi
}

function buildpackage::create::individual() {
  local version buildpack_type combined command artifact_flag
  version="${1}"
  buildpack_type="${2}"

  combined="$(jq -r '.combined' "${CONFIG}")"
  command="$(jq -r '."libpak-tools".command' "${CONFIG}")"
  artifact_flag="$(jq -r '."libpak-tools"."artifact-flag"' "${CONFIG}")"

  util::print::title "Packaging ${buildpack_type}..."

  local args=()
  while IFS= read -r name; do
    args+=(
      --buildpack-id "paketo-buildpacks/${name}@${version}"
      "--${artifact_flag}" "${name}"
    )
  done < <(jq -r --arg combined "${combined}" '.buildpacks[].name | select(. != $combined)' "${CONFIG}")

  if [[ ${#args[@]} -eq 0 ]]; then
    util::print::error "no component buildpacks configured in ${CONFIG}"
  fi

  libpak-tools "${command}" "${args[@]}" \
    --format file \
    --buildpack-path . \
    --output-path "${BUILD_DIR}"
}

function buildpackage::create::combined() {
  local version buildpack_type combined command include_all_flag
  version="${1}"
  buildpack_type="${2}"

  combined="$(jq -r '.combined' "${CONFIG}")"
  command="$(jq -r '."libpak-tools".command' "${CONFIG}")"
  include_all_flag="$(jq -r '."libpak-tools"."include-all-flag"' "${CONFIG}")"

  util::print::title "Packaging ${buildpack_type}..."

  libpak-tools "${command}" \
    --single-buildpack \
    --buildpack-id "paketo-buildpacks/${combined}@${version}" \
    "--${include_all_flag}" \
    --format file \
    --buildpack-path . \
    --output-path "${BUILD_DIR}"
}

main "${@:-}"
