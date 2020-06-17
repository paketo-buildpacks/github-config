#!/bin/bash

set -e
set -u
set -o pipefail

readonly ROOT_DIR="$(cd "$(dirname "${0}")/.." && pwd)"

# shellcheck source=language-family/scripts/.util/tools.sh
source "${ROOT_DIR}/language-family/scripts/.util/tools.sh"

function tools::install() {
  util::tools::jam::install \
      --directory "${ROOT_DIR}/.bin"
}

function main() {
  tools::install

  while read -r directory; do
    pushd "${directory}" > /dev/null || return
      go test -v -count=1 .
    popd > /dev/null || return
  done < <(find "${ROOT_DIR}" -type d -name entrypoint ! -path '*/.git/*')
}

main "${@:-}"
