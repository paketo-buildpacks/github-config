#!/usr/bin/env bash

set -eu
set -o pipefail

readonly ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

# shellcheck source=SCRIPTDIR/sanity.sh
source "${ROOT_DIR}/scripts/sanity.sh"

function main() {
  if [  $# -ne 2 ]; then
      echo "usage: $0 <dst-cnb-dir> <implementation|language-family>"; exit 1;
  fi

  target="$1"
  type="$2"

  sanity::check "${ROOT_DIR}"

  [ -d "$target" ] || { echo "$target" dir not found; exit  1; }

  if [ "$type" != "implementation" ] && [ "$type" != "language-family" ]; then
      echo Invalid cnb type; exit 1
  fi

  cp -pR "${ROOT_DIR}/${type}/." "$target"
}

main "${@:-}"
