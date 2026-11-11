#!/usr/bin/env bash
set -euo pipefail
shopt -s inherit_errexit

function main() {
  local directory jam_version libpak_tools_version
  directory="${TOOLS_DIRECTORY:-${HOME}/bin}"
  jam_version="${JAM_VERSION:-}"
  libpak_tools_version="${LIBPAK_TOOLS_VERSION:-}"

  mkdir -p "${directory}"

  jam_version="$(resolve_version "jam" "${jam_version}" "${TOOLS_CONFIG:-}")"
  libpak_tools_version="$(resolve_version "libpaktools" "${libpak_tools_version}" "${TOOLS_CONFIG:-}")"

  if [[ -z "${jam_version}" && -z "${libpak_tools_version}" ]]; then
    echo "unable to determine a tool version from inputs or ${TOOLS_CONFIG:-the tools config file}" >&2
    exit 1
  fi

  if [[ -n "${jam_version}" ]]; then
    install_jam "${jam_version}" "${directory}"
  else
    echo "Skipping jam: no version provided or found"
  fi

  if [[ -n "${libpak_tools_version}" ]]; then
    install_libpak_tools "${libpak_tools_version}" "${directory}"
  else
    echo "Skipping libpak-tools: no version provided or found"
  fi

  if [[ -n "${GITHUB_PATH:-}" ]]; then
    echo "${directory}" >> "${GITHUB_PATH}"
  fi

  if [[ -n "${GITHUB_OUTPUT:-}" ]]; then
    echo "directory=${directory}" >> "${GITHUB_OUTPUT}"
  fi
}

function resolve_version() {
  local key input config version
  key="${1}"
  input="${2}"
  config="${3}"

  if [[ -n "${input}" ]]; then
    echo "${input#v}"
    return
  fi

  if [[ -n "${config}" && -f "${config}" ]]; then
    version="$(jq -r --arg key "${key}" '.[$key] // empty' "${config}")"
    echo "${version#v}"
    return
  fi

  echo ""
}

function install_jam() {
  local version directory os arch
  version="${1#v}"
  directory="${2}"
  os="$(tool_os)"
  arch="$(tool_arch)"

  echo "Installing jam v${version}"
  download \
    "https://github.com/paketo-buildpacks/jam/releases/download/v${version}/jam-${os}-${arch}" \
    "${directory}/jam"
  chmod +x "${directory}/jam"
}

function install_libpak_tools() {
  local version directory os arch archive
  version="${1#v}"
  directory="${2}"
  os="$(tool_os_capitalized)"
  arch="$(libpak_tools_arch)"
  archive="$(mktemp "${TMPDIR:-/tmp}/libpak-tools.XXXXXX")"

  echo "Installing libpak-tools v${version}"
  download \
    "https://github.com/paketo-buildpacks/libpak-tools/releases/download/v${version}/libpak-tools_${os}_${arch}.tar.gz" \
    "${archive}"
  tar -xzf "${archive}" -C "${directory}" libpak-tools
  rm -f "${archive}"
  chmod +x "${directory}/libpak-tools"
}

function download() {
  local url output
  url="${1}"
  output="${2}"

  curl \
    --fail \
    --show-error \
    --silent \
    --location \
    --retry 3 \
    --connect-timeout 5 \
    --max-time 120 \
    --output "${output}" \
    "${url}"
}

function tool_os() {
  case "$(uname -s)" in
    "Darwin")
      echo "darwin"
      ;;

    "Linux")
      echo "linux"
      ;;

    *)
      echo "unsupported operating system: $(uname -s)" >&2
      exit 1
      ;;
  esac
}

function tool_os_capitalized() {
  case "$(uname -s)" in
    "Darwin")
      echo "Darwin"
      ;;

    "Linux")
      echo "Linux"
      ;;

    *)
      echo "unsupported operating system: $(uname -s)" >&2
      exit 1
      ;;
  esac
}

function tool_arch() {
  case "$(uname -m)" in
    "amd64"|"x86_64")
      echo "amd64"
      ;;

    "arm64"|"aarch64")
      echo "arm64"
      ;;

    "s390x")
      echo "s390x"
      ;;

    *)
      echo "unsupported architecture: $(uname -m)" >&2
      exit 1
      ;;
  esac
}

function libpak_tools_arch() {
  case "$(uname -m)" in
    "amd64"|"x86_64")
      echo "x86_64"
      ;;

    "arm64"|"aarch64")
      echo "arm64"
      ;;

    *)
      echo "unsupported architecture for libpak-tools: $(uname -m)" >&2
      exit 1
      ;;
  esac
}

main "${@:-}"
