#!/bin/sh

set -e

INSTALL_DIR="${HOME}/.local/bin"
BIN_NAME="runic"
REPO="gera2ld/runic"

mkdir -p "$INSTALL_DIR"

detect_os() {
    case "$(uname -s)" in
        Darwin*) echo "darwin" ;;
        Linux*) echo "linux" ;;
        *) echo "linux" ;;
    esac
}

detect_arch() {
    case "$(uname -m)" in
        x86_64|amd64) echo "amd64" ;;
        arm64|aarch64) echo "arm64" ;;
        *) echo "amd64" ;;
    esac
}

get_latest_version() {
    curl -sL "https://github.com/${REPO}/releases/download/latest/version.txt"
}

get_local_version() {
    if [ -x "${INSTALL_DIR}/${BIN_NAME}" ]; then
        "${INSTALL_DIR}/${BIN_NAME}" --version 2>/dev/null || echo ""
    else
        echo ""
    fi
}

OS=$(detect_os)
ARCH=$(detect_arch)
LATEST_VERSION=$(get_latest_version)
LOCAL_VERSION=$(get_local_version)

if [ "$LOCAL_VERSION" = "$LATEST_VERSION" ]; then
    echo "runic ${LATEST_VERSION} is already installed"
    exit 0
fi

if [ -n "$LOCAL_VERSION" ]; then
    echo "Updating runic: ${LOCAL_VERSION} -> ${LATEST_VERSION}"
else
    echo "Installing runic ${LATEST_VERSION}"
fi

URL="https://github.com/${REPO}/releases/download/latest/runic-${OS}-${ARCH}"
DEST="${INSTALL_DIR}/${BIN_NAME}"

echo "Downloading ${URL}"
curl -#L "$URL" -o "$DEST"
chmod 755 "$DEST"

echo "Installed to ${DEST}"
echo "Add to PATH (bash/zsh):"
echo "  export PATH=\"\$HOME/.local/bin:\$PATH\""
