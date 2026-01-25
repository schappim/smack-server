#!/bin/bash
set -e

# smack-server Release Script
# Usage: ./scripts/release.sh [version]
# Example: ./scripts/release.sh v1.1.0

REPO="schappim/smack-server"
BINARY_NAME="smack-server"
DIST_DIR="dist"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

info() { printf "${GREEN}[INFO]${NC} %s\n" "$1"; }
warn() { printf "${YELLOW}[WARN]${NC} %s\n" "$1"; }
error() { printf "${RED}[ERROR]${NC} %s\n" "$1"; exit 1; }
step() { printf "${BLUE}[STEP]${NC} %s\n" "$1"; }

PLATFORMS=(
    "linux/amd64"
    "linux/arm64"
    "darwin/amd64"
    "darwin/arm64"
    "windows/amd64"
)

get_version() {
    if [ -n "$1" ]; then
        VERSION="$1"
    else
        LATEST=$(git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.0")
        echo ""
        echo "  ${BINARY_NAME} Release"
        echo "  $(printf '=%.0s' $(seq 1 $((${#BINARY_NAME} + 8))))"
        echo ""
        echo "  Latest version: ${LATEST}"
        read -p "  Enter new version (e.g., v1.1.0): " VERSION
    fi

    if [[ ! "$VERSION" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
        error "Invalid version format. Use semantic versioning (e.g., v1.0.0)"
    fi

    if git rev-parse "$VERSION" >/dev/null 2>&1; then
        error "Version $VERSION already exists"
    fi
}

check_prereqs() {
    step "Checking prerequisites..."
    command -v go &>/dev/null || error "Go is not installed"
    command -v gh &>/dev/null || error "GitHub CLI (gh) is not installed"
    gh auth status &>/dev/null || error "Not authenticated with GitHub. Run 'gh auth login'"

    if [ -n "$(git status --porcelain)" ]; then
        warn "You have uncommitted changes"
        read -p "  Continue anyway? (y/N): " CONTINUE
        [[ "$CONTINUE" =~ ^[Yy]$ ]] || exit 1
    fi
    info "Prerequisites OK"
}

build_binaries() {
    step "Building binaries..."
    rm -rf "$DIST_DIR"
    mkdir -p "$DIST_DIR"

    for PLATFORM in "${PLATFORMS[@]}"; do
        OS="${PLATFORM%/*}"
        ARCH="${PLATFORM#*/}"
        OUTPUT="${DIST_DIR}/${BINARY_NAME}-${OS}-${ARCH}"
        [ "$OS" = "windows" ] && OUTPUT="${OUTPUT}.exe"

        info "Building ${OS}/${ARCH}..."
        CGO_ENABLED=0 GOOS="$OS" GOARCH="$ARCH" go build -ldflags="-s -w" -o "$OUTPUT" .
    done
    info "All binaries built"
}

create_checksums() {
    step "Creating checksums..."
    cd "$DIST_DIR"
    shasum -a 256 ${BINARY_NAME}-* > checksums.txt
    cd ..
    info "Checksums:"
    cat "$DIST_DIR/checksums.txt"
}

create_release() {
    step "Creating git tag ${VERSION}..."
    git tag -a "$VERSION" -m "Release ${VERSION}"

    step "Pushing to GitHub..."
    git push origin main --tags

    step "Creating GitHub release..."

    NOTES="## ${BINARY_NAME} ${VERSION}

### Installation

#### Homebrew (macOS/Linux)
\`\`\`bash
brew tap schappim/smack-server
brew install smack-server
\`\`\`

#### Direct Download
\`\`\`bash
curl -fsSL https://raw.githubusercontent.com/${REPO}/main/install.sh | sh
\`\`\`

### Checksums (SHA-256)
\`\`\`
$(cat ${DIST_DIR}/checksums.txt)
\`\`\`"

    gh release create "$VERSION" \
        ${DIST_DIR}/${BINARY_NAME}-linux-amd64 \
        ${DIST_DIR}/${BINARY_NAME}-linux-arm64 \
        ${DIST_DIR}/${BINARY_NAME}-darwin-amd64 \
        ${DIST_DIR}/${BINARY_NAME}-darwin-arm64 \
        ${DIST_DIR}/${BINARY_NAME}-windows-amd64.exe \
        ${DIST_DIR}/checksums.txt \
        --title "${VERSION}" \
        --notes "$NOTES"

    info "Release created: https://github.com/${REPO}/releases/tag/${VERSION}"
}

main() {
    get_version "$1"
    check_prereqs
    build_binaries
    create_checksums
    create_release
    echo ""
    echo "  Release ${VERSION} published!"
    echo "  https://github.com/${REPO}/releases/tag/${VERSION}"
    echo ""
    echo "  Next: Update Homebrew formula with ./scripts/update-homebrew.sh ${VERSION}"
    echo ""
}

main "$@"
