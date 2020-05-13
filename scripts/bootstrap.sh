#!/usr/bin/env bash
# Use to bootstrap cnbs with shared files
set -euo pipefail

if [  $# -ne 2 ]; then
    echo "usage: $0 <dst-cnb-dir> <implementation|language-family>"; exit 1;
fi

target="$1"
type="$2"

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
source "$DIR"/sanity.sh

check_sanity

[ -d "$target" ] || { echo "$target" dir not found; exit  1; }

if [ "$type" != "implementation" ] && [ "$type" != "language-family" ]; then
    echo Invalid cnb type; exit 1
fi

cp -pR "${DIR}/../${type}/." "$target"
