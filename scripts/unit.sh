#!/bin/bash

set -e
set -u
set -o pipefail

readonly ROOT_DIR="$(cd "$(dirname "${0}")/.." && pwd)"

function main() {
  while read -r directory; do
    pushd "${directory}" > /dev/null || return
      go test -v -count=1 .
    popd > /dev/null || return
  done < <(find "${ROOT_DIR}" -type d -name entrypoint)
}

main "${@:-}"
