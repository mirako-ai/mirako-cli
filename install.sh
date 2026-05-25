#!/usr/bin/env bash
set -euo pipefail

APP_NAME="mirako"
REPO="mirako-ai/mirako-cli"

INSTALL_DIR="${MIRAKO_INSTALL_DIR:-$HOME/.mirako/bin}"
REQUESTED_VERSION="${MIRAKO_VERSION:-}"
NO_MODIFY_PATH=false
path_was_present=false

RED='\033[0;31m'
YELLOW='\033[0;33m'
DIM='\033[0;2m'
RESET='\033[0m'

usage() {
  cat <<EOF
Mirako CLI installer

Usage: install.sh [options]

Options:
  -h, --help                Show this help message
  -v, --version <version>   Install a specific version (for example: 1.2.1)
  -d, --install-dir <path>  Install directory (default: ~/.mirako/bin)
      --no-modify-path      Do not add the install directory to your shell PATH

Environment variables:
  MIRAKO_VERSION            Same as --version
  MIRAKO_INSTALL_DIR        Same as --install-dir

Examples:
  curl -fsSL https://raw.githubusercontent.com/mirako-ai/mirako-cli/main/install.sh | bash
  curl -fsSL https://raw.githubusercontent.com/mirako-ai/mirako-cli/main/install.sh | bash -s -- --version 1.2.1
  curl -fsSL https://raw.githubusercontent.com/mirako-ai/mirako-cli/main/install.sh | bash -s -- --install-dir /usr/local/bin
EOF
}

info() {
  printf "%b\n" "${DIM}$1${RESET}"
}

warn() {
  printf "%b\n" "${YELLOW}Warning:${RESET} $1" >&2
}

fail() {
  printf "%b\n" "${RED}Error:${RESET} $1" >&2
  exit 1
}

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || fail "'$1' is required but was not found"
}

parse_args() {
  while [[ $# -gt 0 ]]; do
    case "$1" in
      -h|--help)
        usage
        exit 0
        ;;
      -v|--version)
        [[ -n "${2:-}" ]] || fail "--version requires a value"
        REQUESTED_VERSION="$2"
        shift 2
        ;;
      -d|--install-dir)
        [[ -n "${2:-}" ]] || fail "--install-dir requires a value"
        INSTALL_DIR="$2"
        shift 2
        ;;
      --no-modify-path)
        NO_MODIFY_PATH=true
        shift
        ;;
      *)
        fail "unknown option: $1"
        ;;
    esac
  done
}

resolve_tag() {
  if [[ -n "$REQUESTED_VERSION" ]]; then
    TAG="v${REQUESTED_VERSION#v}"
    return
  fi

  local latest_url
  latest_url="$(curl -fsSL -o /dev/null -w '%{url_effective}' "https://github.com/${REPO}/releases/latest")"
  TAG="${latest_url##*/}"
  [[ "$TAG" == v* ]] || fail "could not determine the latest release tag"
}

detect_platform() {
  local raw_os raw_arch translated

  raw_os="$(uname -s)"
  case "$raw_os" in
    Darwin)
      asset_os="Darwin"
      archive_ext="tar.gz"
      ;;
    Linux)
      asset_os="Linux"
      archive_ext="tar.gz"
      ;;
    MINGW*|MSYS*|CYGWIN*)
      asset_os="Windows"
      archive_ext="zip"
      ;;
    *)
      fail "unsupported operating system: $raw_os"
      ;;
  esac

  raw_arch="$(uname -m)"
  case "$raw_arch" in
    x86_64|amd64)
      asset_arch="x86_64"
      ;;
    arm64|aarch64)
      asset_arch="arm64"
      ;;
    *)
      fail "unsupported architecture: $raw_arch"
      ;;
  esac

  if [[ "$asset_os" == "Darwin" && "$asset_arch" == "x86_64" ]]; then
    translated="$(sysctl -n sysctl.proc_translated 2>/dev/null || true)"
    if [[ "$translated" == "1" ]]; then
      asset_arch="arm64"
    fi
  fi

  if [[ "$asset_os" == "Windows" && "$asset_arch" == "arm64" ]]; then
    fail "Windows arm64 is not published yet"
  fi

  asset_name="mirako-cli_${asset_os}_${asset_arch}.${archive_ext}"
  binary_name="$APP_NAME"
  if [[ "$asset_os" == "Windows" ]]; then
    binary_name="${APP_NAME}.exe"
  fi
}

current_version() {
  local installed_binary="$INSTALL_DIR/$binary_name"
  if [[ -x "$installed_binary" ]]; then
    "$installed_binary" --version 2>/dev/null | tr -d '\r'
  fi
}

download_release() {
  local base_url="https://github.com/${REPO}/releases/download/${TAG}"

  archive_path="$tmp_dir/$asset_name"
  checksums_path="$tmp_dir/checksums.txt"

  info "Installing ${APP_NAME} ${TAG#v} for ${asset_os}/${asset_arch}"
  curl -fsSL "$base_url/$asset_name" -o "$archive_path"
  curl -fsSL "$base_url/checksums.txt" -o "$checksums_path"
}

verify_checksum() {
  local checksum_line expected actual

  checksum_line="$(grep -F "  $asset_name" "$checksums_path" || true)"
  [[ -n "$checksum_line" ]] || fail "checksum for $asset_name was not found"
  expected="${checksum_line%% *}"

  if command -v sha256sum >/dev/null 2>&1; then
    actual="$(sha256sum "$archive_path" | awk '{print $1}')"
  elif command -v shasum >/dev/null 2>&1; then
    actual="$(shasum -a 256 "$archive_path" | awk '{print $1}')"
  else
    warn "Skipping checksum verification because neither sha256sum nor shasum is available"
    return
  fi

  [[ "$actual" == "$expected" ]] || fail "checksum verification failed for $asset_name"
}

extract_archive() {
  if [[ "$archive_ext" == "tar.gz" ]]; then
    require_cmd tar
    tar -xzf "$archive_path" -C "$tmp_dir"
    return
  fi

  require_cmd unzip
  unzip -q "$archive_path" -d "$tmp_dir"
}

ensure_install_dir() {
  if mkdir -p "$INSTALL_DIR" 2>/dev/null; then
    use_sudo=false
    return
  fi

  command -v sudo >/dev/null 2>&1 || fail "cannot create $INSTALL_DIR and sudo is not available"
  sudo mkdir -p "$INSTALL_DIR"
  use_sudo=true
}

install_binary() {
  local extracted_binary="$tmp_dir/$binary_name"
  [[ -f "$extracted_binary" ]] || fail "downloaded archive did not contain $binary_name"

  ensure_install_dir

  if [[ "$use_sudo" == true || ! -w "$INSTALL_DIR" ]]; then
    command -v sudo >/dev/null 2>&1 || fail "cannot write to $INSTALL_DIR and sudo is not available"
    sudo install -m 755 "$extracted_binary" "$INSTALL_DIR/$binary_name"
    use_sudo=true
    return
  fi

  install -m 755 "$extracted_binary" "$INSTALL_DIR/$binary_name"
}

path_contains_install_dir() {
  case ":$PATH:" in
    *":$INSTALL_DIR:"*) return 0 ;;
    *) return 1 ;;
  esac
}

append_path_entry() {
  local shell_name config_file path_line
  shell_name="$(basename "${SHELL:-}")"

  case "$shell_name" in
    fish)
      config_file="${XDG_CONFIG_HOME:-$HOME/.config}/fish/config.fish"
      path_line="fish_add_path \"$INSTALL_DIR\""
      ;;
    zsh)
      config_file="${ZDOTDIR:-$HOME}/.zshrc"
      path_line="export PATH=\"$INSTALL_DIR:\$PATH\""
      ;;
    bash)
      if [[ -f "$HOME/.bashrc" || ! -f "$HOME/.bash_profile" ]]; then
        config_file="$HOME/.bashrc"
      else
        config_file="$HOME/.bash_profile"
      fi
      path_line="export PATH=\"$INSTALL_DIR:\$PATH\""
      ;;
    sh|ash|dash)
      config_file="$HOME/.profile"
      path_line="export PATH=\"$INSTALL_DIR:\$PATH\""
      ;;
    *)
      warn "Could not determine which shell config to update"
      printf 'Add this to your shell config:\n  export PATH="%s:$PATH"\n' "$INSTALL_DIR"
      return
      ;;
  esac

  mkdir -p "$(dirname "$config_file")"
  touch "$config_file"

  if grep -Fqx "$path_line" "$config_file"; then
    info "$config_file already contains the Mirako PATH entry"
    return
  fi

  printf '\n# mirako\n%s\n' "$path_line" >> "$config_file"
  info "Added $INSTALL_DIR to PATH in $config_file"
}

print_success() {
  printf '\n'
  printf 'Mirako CLI is installed in %s\n' "$INSTALL_DIR"
  if [[ "$path_was_present" == false ]]; then
    printf 'Open a new shell or run: export PATH="%s:$PATH"\n' "$INSTALL_DIR"
  fi
  printf 'Verify with: %s --version\n' "$APP_NAME"
}

main() {
  local installed_version

  parse_args "$@"
  require_cmd curl
  require_cmd grep
  require_cmd awk
  require_cmd install

  resolve_tag
  detect_platform

  if path_contains_install_dir; then
    path_was_present=true
  fi

  tmp_dir="$(mktemp -d 2>/dev/null || mktemp -d -t mirako-install)"
  trap 'rm -rf "$tmp_dir"' EXIT

  installed_version="$(current_version || true)"
  if [[ "$installed_version" == "${TAG#v}" ]]; then
    info "${APP_NAME} ${installed_version} is already installed in $INSTALL_DIR"
  else
    download_release
    verify_checksum
    extract_archive
    install_binary
  fi

  if [[ "$NO_MODIFY_PATH" == false ]] && ! path_contains_install_dir; then
    append_path_entry
  fi

  print_success
}

main "$@"
