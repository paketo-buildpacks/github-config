#!/usr/bin/env bash
# Checks if the contents of implementation/ and language-family/ follow rules
# Future TODO: Run this on every PR
set -euo pipefail

ROOTDIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )/.." && pwd )"

check_sanity () {
    # Rule 1 - all children of implemenation/ & language-family/ must be directories
    for cnbdir in 'implementation' 'language-family' ; do
        [ -d "$ROOTDIR"/"$cnbdir" ] || { echo "$cnbdir" dir not found; exit  1; }
        if [ $(ls -Apq "$ROOTDIR"/"$cnbdir" | grep -v /) ]; then
            echo All files in "$cnbdir/" must be directories. Exiting.
            exit 1
        fi
    done
}

check_sanity "$@"
