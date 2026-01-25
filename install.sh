#!/bin/sh
set -e

# smack-server Installer
# Usage: curl -fsSL https://raw.githubusercontent.com/schappim/smack-server/main/install.sh | sh

REPO="schappim/smack-server"
BINARY_NAME="smack-server"
INSTALL_DIR="/usr/local/bin"

# Colors (POSIX-compatible)
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

info() { printf "${GREEN}[INFO]${NC} %s\n" "$1"; }
warn() { printf "${YELLOW}[WARN]${NC} %s\n" "$1"; }
error() { printf "${RED}[ERROR]${NC} %s\n" "$1"; exit 1; }

detect_os() {
    case "$(uname -s)" in
        Linux*)  echo "linux" ;;
        Darwin*) echo "darwin" ;;
        MINGW*|MSYS*|CYGWIN*) echo "windows" ;;
        *) error "Unsupported OS: $(uname -s)" ;;
    esac
}

detect_arch() {
    case "$(uname -m)" in
        x86_64|amd64)  echo "amd64" ;;
        arm64|aarch64) echo "arm64" ;;
        *) error "Unsupported architecture: $(uname -m)" ;;
    esac
}

get_latest_version() {
    curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | \
        grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/'
}

main() {
    echo ""
    echo "  smack-server Installer"
    echo "  ======================"
    echo ""

    OS=$(detect_os)
    ARCH=$(detect_arch)
    info "Detected: ${OS}/${ARCH}"

    info "Fetching latest version..."
    VERSION=$(get_latest_version)
    [ -z "$VERSION" ] && error "Could not determine latest version"
    info "Latest version: ${VERSION}"

    EXT=""
    [ "$OS" = "windows" ] && EXT=".exe"

    FILENAME="${BINARY_NAME}-${OS}-${ARCH}${EXT}"
    URL="https://github.com/${REPO}/releases/download/${VERSION}/${FILENAME}"

    info "Downloading ${FILENAME}..."
    TMP_DIR=$(mktemp -d)
    TMP_FILE="${TMP_DIR}/${BINARY_NAME}${EXT}"

    if ! curl -fsSL "$URL" -o "$TMP_FILE"; then
        rm -rf "$TMP_DIR"
        error "Download failed: ${URL}"
    fi

    chmod +x "$TMP_FILE"

    if [ "$OS" = "windows" ]; then
        INSTALL_DIR="$HOME/bin"
        mkdir -p "$INSTALL_DIR"
    fi

    info "Installing to ${INSTALL_DIR}/${BINARY_NAME}${EXT}..."

    if [ -w "$INSTALL_DIR" ]; then
        mv "$TMP_FILE" "${INSTALL_DIR}/${BINARY_NAME}${EXT}"
    else
        warn "Need sudo to install to ${INSTALL_DIR}"
        sudo mv "$TMP_FILE" "${INSTALL_DIR}/${BINARY_NAME}${EXT}"
    fi

    rm -rf "$TMP_DIR"

    echo ""
    info "Successfully installed ${BINARY_NAME} ${VERSION}!"
    echo ""
    echo "  Run '${BINARY_NAME}' to start the server"
    echo ""
}

main
