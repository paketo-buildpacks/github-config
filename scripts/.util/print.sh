#!/usr/bin/env bash

set -eu
set -o pipefail

function util::print::title() {
  local message
  message="${1}"

  util::print::blue "\n${message}"
}

function util::print::info() {
  local message
  message="${1}"

  echo -e "${message}" >&2
}

function util::print::error() {
  local message
  message="${1}"

  util::print::red "${message}"
  exit 1
}

function util::print::success() {
  local message
  message="${1}"

  util::print::green "${message}"
  exit 0
}

function util::print::warn() {
  local message
  message="${1}"

  util::print::yellow "${message}"
  exit 0
}

function util::print::blue() {
  local message blue reset
  message="${1}"
  blue="\033[0;34m"
  reset="\033[0;39m"

  echo -e "${blue}${message}${reset}" >&2
}

function util::print::red() {
  local message red reset
  message="${1}"
  red="\033[0;31m"
  reset="\033[0;39m"

  echo -e "${red}${message}${reset}" >&2
}

function util::print::green() {
  local message green reset
  message="${1}"
  green="\033[0;32m"
  reset="\033[0;39m"

  echo -e "${green}${message}${reset}" >&2
}

function util::print::yellow() {
  local message yellow reset
  message="${1}"
  yellow="\033[0;33m"
  reset="\033[0;39m"

  echo -e "${yellow}${message}${reset}" >&2
}

function util::print::break() {
  echo "" >&2
}

function util::print::indent() {
  sed 's/^.*\\r//g' | sed 's/^/  /g'
}
