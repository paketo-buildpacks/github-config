#!/usr/bin/env bash
set -euo pipefail
shopt -s inherit_errexit


usage() {
  cat <<EOF
Usage: generate-package-list.sh --build-receipt <path> --run-receipt <path> [--output <path>]

Generates a JSON array of package names from build and run image receipts.

Options:
  --build-receipt <path>  Package receipt for build image (required)
  --run-receipt <path>    Package receipt for run image (required)
  --output <path>         Path to write output (optional, prints to stdout if omitted)
  -h, --help              Show this help message

Examples:
  generate-package-list.sh --build-receipt build-receipt.json --run-receipt run-receipt.json
  generate-package-list.sh --build-receipt build-receipt.json --run-receipt run-receipt.json --output packages.json
EOF
}

main() {
  local build_receipt=""
  local run_receipt=""
  local output_path=""

  while [[ $# -gt 0 ]]; do
    case "$1" in
      --build-receipt)
        build_receipt="$2"
        shift 2
        ;;
      --run-receipt)
        run_receipt="$2"
        shift 2
        ;;
      --output)
        output_path="$2"
        shift 2
        ;;
      -h|--help)
        usage
        exit 0
        ;;
      *)
        echo "Error: Unknown option: $1" >&2
        usage >&2
        exit 1
        ;;
    esac
  done

  if [[ -z "$build_receipt" ]]; then
    echo "Error: --build-receipt is required" >&2
    usage >&2
    exit 1
  fi

  if [[ -z "$run_receipt" ]]; then
    echo "Error: --run-receipt is required" >&2
    usage >&2
    exit 1
  fi

  if [[ ! -f "$build_receipt" ]]; then
    echo "Error: Build receipt file not found: $build_receipt" >&2
    exit 1
  fi

  if [[ ! -f "$run_receipt" ]]; then
    echo "Error: Run receipt file not found: $run_receipt" >&2
    exit 1
  fi

  packages=$(jq '.components[] | .name' "$build_receipt" "$run_receipt" \
    | jq --null-input --compact-output '[inputs]')

  if [[ -n "$output_path" ]]; then
    echo "${packages}" > "$output_path"
    echo "$output_path"
  else
    echo "${packages}"
  fi
}

main "$@"
