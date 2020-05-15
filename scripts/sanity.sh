#!/usr/bin/env bash
# Checks if the contents of implementation/ and language-family/ follow rules
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

    # Rule 2 - Data files must be single line records of repo names
    for datafile in 'implementation-cnbs' 'language-family-cnbs' ; do
        lnum=1
        while read line; do
            if [[ ! "$line" =~ paketo-buildpacks/[A-Za-z0-9_.-] ]]; then
                echo "Invalid data file "$datafile". (line: $lnum)"
                exit 1
            fi
            lnum=$((lnum+1))
        done < "${ROOTDIR}/.github/data/$datafile"
    done
}

check_sanity "$@"
