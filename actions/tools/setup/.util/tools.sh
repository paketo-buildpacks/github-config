#!/usr/bin/env bash

set -eu
set -o pipefail

# shellcheck source=SCRIPTDIR/print.sh
source "$(dirname "${BASH_SOURCE[0]}")/print.sh"

function util::tools::os() {
  case "$(uname)" in
    "Darwin")
      if [[ "${1:-}" == "--capitalized" ]]; then
        echo "Darwin"
      else
        echo "${1:-darwin}"
      fi
      ;;

    "Linux")
      if [[ "${1:-}" == "--capitalized" ]]; then
        echo "Linux"
      else
        echo "linux"
      fi
      ;;

    *)
      util::print::error "Unknown OS \"$(uname)\""
      exit 1
  esac
}

function util::tools::arch() {
  case "$(uname -m)" in
    arm64|aarch64)
      echo "arm64"
      ;;

    amd64|x86_64)
      if [[ "${1:-}" == "--format-amd64-x86_64" ]]; then
        echo "x86_64"
      else
        echo "amd64"
      fi
      ;;

    s390x)
      echo "s390x"
      ;;

    *)
      util::print::error "Unknown Architecture \"$(uname -m)\""
      exit 1
  esac
}

function util::tools::path::export() {
  local dir
  dir="${1}"

  if ! echo "${PATH}" | grep -q "${dir}"; then
    PATH="${dir}:$PATH"
    export PATH
  fi
}

function util::tools::version::resolve() {
  local key input config version
  key="${1}"
  input="${2}"
  config="${3}"

  if [[ -n "${input}" ]]; then
    echo "v${input#v}"
    return
  fi

  if [[ -n "${config}" && -f "${config}" ]]; then
    version="$(jq -r --arg key "${key}" '.[$key] // empty' "${config}")"
    if [[ -n "${version}" ]]; then
      echo "v${version#v}"
    else
      echo ""
    fi
    return
  fi

  echo ""
}

function util::tools::jam::install() {
  local dir token version curl_args os arch tools_json
  token=""
  version=""

  while [[ "${#}" != 0 ]]; do
    case "${1}" in
      --directory)
        dir="${2}"
        shift 2
        ;;

      --token)
        token="${2}"
        shift 2
        ;;

      --version)
        version="${2}"
        shift 2
        ;;

      *)
        util::print::error "unknown argument \"${1}\""
    esac
  done

  mkdir -p "${dir}"
  util::tools::path::export "${dir}"

  if [[ -z "${version}" ]]; then
    tools_json="$(dirname "${BASH_SOURCE[0]}")/tools.json"
    if [[ -f "${tools_json}" ]]; then
      version="$(jq -r .jam "${tools_json}")"
    fi
  fi

  if [[ -z "${version}" ]]; then
    util::print::error "jam version is required"
  fi
  version="v${version#v}"

  curl_args=(
    "--fail"
    "--show-error"
    "--silent"
    "--location"
    "--retry" "3"
    "--connect-timeout" "5"
    "--max-time" "120"
    "--output" "${dir}/jam"
  )

  if [[ "${token}" != "" ]]; then
    curl_args+=("--header" "Authorization: Token ${token}")
  fi

  util::print::title "Installing jam ${version}"

  os=$(util::tools::os)
  arch=$(util::tools::arch)

  curl "https://github.com/paketo-buildpacks/jam/releases/download/${version}/jam-${os}-${arch}" \
    "${curl_args[@]}"

  chmod +x "${dir}/jam"
}

function util::tools::libpak-tools::install() {
  local dir token version curl_args os arch archive tools_json
  token=""
  version=""

  while [[ "${#}" != 0 ]]; do
    case "${1}" in
      --directory)
        dir="${2}"
        shift 2
        ;;

      --token)
        token="${2}"
        shift 2
        ;;

      --version)
        version="${2}"
        shift 2
        ;;

      *)
        util::print::error "unknown argument \"${1}\""
    esac
  done

  mkdir -p "${dir}"
  util::tools::path::export "${dir}"

  if [[ -z "${version}" ]]; then
    tools_json="$(dirname "${BASH_SOURCE[0]}")/tools.json"
    if [[ -f "${tools_json}" ]]; then
      version="$(jq -r .libpaktools "${tools_json}")"
    fi
  fi

  if [[ -z "${version}" ]]; then
    util::print::error "libpak-tools version is required"
  fi
  version="v${version#v}"

  curl_args=(
    "--fail"
    "--show-error"
    "--silent"
    "--location"
    "--retry" "3"
    "--connect-timeout" "5"
    "--max-time" "120"
  )

  if [[ "${token}" != "" ]]; then
    curl_args+=("--header" "Authorization: Token ${token}")
  fi

  util::print::title "Installing libpak-tools ${version}"

  os=$(util::tools::os --capitalized)
  arch=$(util::tools::arch --format-amd64-x86_64)

  case "${arch}" in
    x86_64|arm64)
      ;;
    *)
      util::print::error "unsupported architecture for libpak-tools: $(uname -m)"
      ;;
  esac

  archive="$(mktemp "${TMPDIR:-/tmp}/libpak-tools.XXXXXX")"
  curl_args+=("--output" "${archive}")

  curl "https://github.com/paketo-buildpacks/libpak-tools/releases/download/${version}/libpak-tools_${os}_${arch}.tar.gz" \
    "${curl_args[@]}"

  tar -xzf "${archive}" -C "${dir}" libpak-tools
  rm -f "${archive}"

  chmod +x "${dir}/libpak-tools"
}
