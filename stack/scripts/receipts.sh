#!/usr/bin/env bash

set -eu
set -o pipefail

readonly PROG_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly STACK_DIR="$(cd "${PROG_DIR}/.." && pwd)"
readonly BIN_DIR="${STACK_DIR}/.bin"
readonly BUILD_DIR="${STACK_DIR}/build"

# shellcheck source=SCRIPTDIR/.util/tools.sh
source "${PROG_DIR}/.util/tools.sh"

# shellcheck source=SCRIPTDIR/.util/print.sh
source "${PROG_DIR}/.util/print.sh"

function main() {
  local build run buildReceipt runReceipt
  build="${BUILD_DIR}/build.oci"
  run="${BUILD_DIR}/run.oci"
  buildReceipt="${BUILD_DIR}/build-receipt.txt"
  runReceipt="${BUILD_DIR}/run-receipt.txt"

  while [[ "${#}" != 0 ]]; do
    case "${1}" in
      --help|-h)
        shift 1
        usage
        exit 0
        ;;

      --build-archive|-b)
        build="${2}"
        shift 2
        ;;

      --run-archive|-r)
        run="${2}"
        shift 2
        ;;

      --build-receipt|-B)
        buildReceipt="${2}"
        shift 2
        ;;

      --run-receipt|-R)
        runReceipt="${2}"
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

  tools::install

  receipts::generate "${build}" "${buildReceipt}"
  receipts::generate "${run}" "${runReceipt}"

  util::print::success "Success! Receipts are:\n  ${buildReceipt}\n  ${runReceipt}\n"
}

function usage() {
  cat <<-USAGE
receipts.sh [OPTIONS]

Generates receipts listing packages installed on build and run images of the
stack.

OPTIONS
  --help          -h  prints the command usage
  --build-archive -b  path to OCI archive of build image. Defaults to
                      ${BUILD_DIR}/build.oci
  --run-archive   -r  path to OCI archive of build image
                      ${BUILD_DIR}/run.oci
  --build-receipt -B  path to output build image package receipt. Defaults to
                      ${BUILD_DIR}/build-receipt.txt
  --run-receipt   -R  path to output run image package receipt. Defaults to
                      ${BUILD_DIR}/run-receipt.txt
USAGE
}

function tools::install() {
  util::tools::skopeo::check
}

function receipts::generate() {
  local archive output hasDpkg

  archive="${1}"
  output="${2}"

  util::print::title "Generating package receipt for ${archive}"

  util::print::info "Moving ${archive} onto the docker daemon"
  skopeo copy oci-archive://"${archive}" docker-daemon:receipts-stack:latest

  set +e
  docker run --rm receipts-stack:latest which dpkg &> /dev/null
  hasDpkg=$?
  set -e

  if [[ "${hasDpkg}" == 0 ]]; then
  util::print::info "Gathering package list using dkpg -l"
    docker run --rm receipts-stack:latest dpkg -l > "${output}"
  else
    util::print::info "Gathering package list using package control file contents"
    container_id="$(docker create receipts-stack:latest sleep)"
    docker cp "${container_id}":/var/lib/dpkg/status.d /tmp/tiny-pkgs
    docker rm -v "${container_id}"

    printf "                   Name                              Version                            Architecture\n" > "${output}"
    printf "+++-===================================-===================================-===================================\n" >> "${output}"

    for pkg in /tmp/tiny-pkgs/* ; do
      name="$(cat "${pkg}" | grep ^Package: | cut -d ' ' -f2)"
      version="$(cat "${pkg}" | grep ^Version: | cut -d ' ' -f2)"
      arch="$(cat "${pkg}" | grep ^Architecture: | cut -d ' ' -f2)"
      printf "ii  %-35s %-35s %-35s\n" "${name}" "${version}" "${arch}" >> "${output}"
    done
    rm -rf /tmp/tiny-pkgs
  fi

  util::print::info "Cleaning up ${archive} from the docker daemon"
  docker rmi -f receipts-stack:latest
}

main "${@:-}"
