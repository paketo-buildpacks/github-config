#!/usr/bin/env bash

set -eu
set -o pipefail

readonly PROG_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly ROOT_DIR="$(cd "${PROG_DIR}/.." && pwd)"
readonly BIN_DIR="${ROOT_DIR}/.bin"

# shellcheck source=SCRIPTDIR/.util/tools.sh
source "${PROG_DIR}/.util/tools.sh"

# shellcheck source=SCRIPTDIR/.util/print.sh
source "${PROG_DIR}/.util/print.sh"

if [[ $BASH_VERSINFO -lt 4 ]]; then
  util::print::error "Before running this script please update Bash to v4 or higher (e.g. on OSX: \$ brew install bash)"
fi

function main() {
  local build_ref=()
  local run_ref=()
  local image_ref=()
  local build_archive=""
  local run_archive=""
  local image_archive=""
  local receipts_dir=""

  while [[ "${#}" != 0 ]]; do
    case "${1}" in
      --help|-h)
        shift 1
        usage
        exit 0
        ;;

      --build-ref)
        build_ref+=("${2}")
        shift 2
        ;;

      --run-ref)
        run_ref+=("${2}")
        shift 2
        ;;

      --build-archive)
        build_archive=${2}
        shift 2
        ;;

      --run-archive)
        run_archive=${2}
        shift 2
        ;;

      --image-ref)
        image_ref+=("${2}")
        shift 2
        ;;

      --image-archive)
        image_archive=${2}
        shift 2
        ;;

      --receipts-dir)
        receipts_dir=${2}
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

  if [[ ${#image_ref[@]} != 0 || -n "$image_archive" ]]; then
    if ((${#image_ref[@]} == 0)); then
      util::print::error "--image-ref is required [Example: docker.io/paketobuildpacks/foo:latest]"
    fi

    if [ -z "$image_archive" ]; then
      util::print::error "--image-archive is required [Example: ./path/to/image.oci]"
    fi
  else
    if ((${#build_ref[@]} == 0)); then
      util::print::error "--build-ref is required [Example: docker.io/paketobuildpacks/foo:latest]"
    fi

    if ((${#run_ref[@]} == 0)); then
      util::print::error "--run-ref is required [Example: docker.io/paketobuildpacks/foo:1.0.0]"
    fi

    if ((${#run_ref[@]} != ${#build_ref[@]})); then
      util::print::error "must have the same number of --build-ref and --run-ref arguments"
    fi

    if [ -z "$build_archive" ]; then
      util::print::error "--build-archive is required [Example: ./path/to/build.oci]"
    fi

    if [ -z "$run_archive" ]; then
      util::print::error "--run-archive is required [Example: ./path/to/run.oci]"
    fi
  fi

  tools::install

  if [[ ${#image_ref[@]} != 0 || -n "$image_archive" ]]; then
    stack::publish::image \
      "$image_archive" \
      "$receipts_dir" \
      "${#image_ref[@]}" \
      "${image_ref[@]}"
  else
    stack::publish \
      "$build_archive" \
      "$run_archive" \
      "${#build_ref[@]}" \
      "${build_ref[@]}" \
      "${#run_ref[@]}" \
      "${run_ref[@]}"
  fi
}

function usage() {
  cat <<-USAGE
publish.sh [OPTIONS]

Publishes the stack using the existing OCI image archives.

OPTIONS
  --build-ref          list of build references to publish to [Required if --image-ref is not provided]
  --run-ref            list of run references to publish to [Required if --image-ref is not provided]
  --build-archive      path to the build OCI archive file [Required if --image-ref is not provided]
  --run-archive        path to the run OCI archive file [Required if --image-ref is not provided]
  --image-ref          list of image references to publish to [Required if --build-ref and --run-ref are not provided]
  --image-archive      path to the image OCI archive file [Required if --build-ref and --run-ref are not provided]
  --receipts-dir       directory of package receipts; the ones of the image are attached to it as SBOM
                       attestations [Optional, with --image-archive]
  --help          -h   prints the command usage
USAGE
}

function tools::install() {
  util::tools::jam::install \
    --directory "${BIN_DIR}"
}

function stack::publish() {
  local build_archive="$1"
  local run_archive="$2"

  # bash can't easily pass arrays, they all get merged into one list of arguments
  #  so we pass the lengths & extract the arrays from the single argument list
  local build_ref_len="$3"                    # length of build ref array
  local build_ref=("${@:4:$build_ref_len}")   # pull out build_ref array
  local run_len_slot=$(( 4 + build_ref_len))  # location of run_ref length
  local run_ref_len="${*:$run_len_slot:1}"    # length of run ref array
  local run_ref_slot=$(( 1 + run_len_slot))   # location of run_ref array
  local run_ref=("${@:$run_ref_slot:$run_ref_len}")  # pull out run_ref array

  # iterate over build_ref & run_ref, they will be the same length
  local len=${#build_ref[@]}
  for (( i=0; i<len; i++ )); do
    local br="${build_ref[$i]}"
    local rr="${run_ref[$i]}"
    args=(
      "--build-ref" "$br"
      "--run-ref" "$rr"
      "--build-archive" "$build_archive"
      "--run-archive" "$run_archive"
    )
    jam publish-stack "${args[@]}"
  done
}

function stack::publish::image() {
  local image_archive="$1"
  local receipts_dir="$2"

  # bash can't easily pass arrays, they all get merged into one list of arguments
  #  so we pass the lengths & extract the arrays from the single argument list
  local image_ref_len="$3"                    # length of image ref array
  local image_ref=("${@:4:$image_ref_len}")   # pull out image_ref array

  local sboms=()
  if [[ -n "$receipts_dir" ]]; then
    if jam publish-image --help 2>&1 | grep -q -- "--sbom"; then
      local sbom
      while IFS= read -r sbom; do
        sboms+=("--sbom" "$sbom")
      done < <(stack::receipts::of::image "$image_archive" "$receipts_dir")
    else
      # not util::print::warn, which exits
      util::print::info "WARNING: this version of jam cannot attach SBOMs to an image, publishing without them"
    fi
  fi

  # iterate over image_ref
  local len=${#image_ref[@]}
  for (( i=0; i<len; i++ )); do
    local ir="${image_ref[$i]}"
    args=(
      "--image-ref" "$ir"
      "--image-archive" "$image_archive"
      "${sboms[@]}"
    )
    jam publish-image "${args[@]}"
  done
}

# Prints <architecture>=<path> for each receipt of the image: <archive name>-receipt.cyclonedx.json for amd64,
# the same with -<architecture> in the name for the others.
function stack::receipts::of::image() {
  local image_archive="$1"
  local receipts_dir="$2"
  local name arch receipt base

  name="$(basename "${image_archive}" .oci)"

  for arch in $(tar -xOf "${image_archive}" index.json | jq -r '.manifests[].platform.architecture // empty' | sort -u); do
    for receipt in "${receipts_dir}"/*-receipt.cyclonedx.json; do
      [[ -e "${receipt}" ]] || continue
      base="$(basename "${receipt}" -receipt.cyclonedx.json)"

      if [[ "${arch}" == "amd64" && "${base}" == "${name}" ]] || [[ "${arch}" != "amd64" && "${base}" != "${name}" && "${base/-${arch}/}" == "${name}" ]]; then
        echo "${arch}=${receipt}"
        continue 2
      fi
    done

    util::print::info "WARNING: no receipt found in ${receipts_dir} for the ${arch} image of ${name}, it gets no SBOM"
  done
}

main "${@:-}"
