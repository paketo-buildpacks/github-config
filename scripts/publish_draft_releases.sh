#!/usr/bin/env bash

set -eu -o pipefail
shopt -s inherit_errexit

readonly BIN_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )/publish_draft_releases" && pwd )"

pushd "${BIN_DIR}" > /dev/null
go run main.go "${@:-}"
