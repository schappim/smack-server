#!/bin/bash
set -e

# Update Homebrew formula after a release
# Usage: ./scripts/update-homebrew.sh v1.0.0

VERSION="${1#v}"  # Remove 'v' prefix if present
[ -z "$VERSION" ] && { echo "Usage: $0 <version>"; exit 1; }

REPO="schappim/smack-server"
TAP_REPO="schappim/homebrew-smack-server"
BINARY="smack-server"

echo "Fetching checksums for v${VERSION}..."

# Download checksums
CHECKSUMS=$(curl -fsSL "https://github.com/${REPO}/releases/download/v${VERSION}/checksums.txt")

# Extract SHA256 for each platform
SHA_DARWIN_ARM64=$(echo "$CHECKSUMS" | grep "darwin-arm64" | awk '{print $1}')
SHA_DARWIN_AMD64=$(echo "$CHECKSUMS" | grep "darwin-amd64" | awk '{print $1}')
SHA_LINUX_ARM64=$(echo "$CHECKSUMS" | grep "linux-arm64" | awk '{print $1}')
SHA_LINUX_AMD64=$(echo "$CHECKSUMS" | grep "linux-amd64" | awk '{print $1}')

echo "SHA256 Checksums:"
echo "  darwin-arm64: ${SHA_DARWIN_ARM64}"
echo "  darwin-amd64: ${SHA_DARWIN_AMD64}"
echo "  linux-arm64:  ${SHA_LINUX_ARM64}"
echo "  linux-amd64:  ${SHA_LINUX_AMD64}"

echo ""
echo "Cloning tap repository..."
TEMP_DIR=$(mktemp -d)
git clone "git@github.com:${TAP_REPO}.git" "$TEMP_DIR"
cd "$TEMP_DIR"

echo "Updating formula..."
FORMULA="Formula/${BINARY}.rb"

# Update version
sed -i '' "s/version \".*\"/version \"${VERSION}\"/" "$FORMULA"

# Update SHA256 hashes (using comments as markers)
sed -i '' "s/sha256 \".*\" # darwin-arm64/sha256 \"${SHA_DARWIN_ARM64}\" # darwin-arm64/" "$FORMULA"
sed -i '' "s/sha256 \".*\" # darwin-amd64/sha256 \"${SHA_DARWIN_AMD64}\" # darwin-amd64/" "$FORMULA"
sed -i '' "s/sha256 \".*\" # linux-arm64/sha256 \"${SHA_LINUX_ARM64}\" # linux-arm64/" "$FORMULA"
sed -i '' "s/sha256 \".*\" # linux-amd64/sha256 \"${SHA_LINUX_AMD64}\" # linux-amd64/" "$FORMULA"

git add "$FORMULA"
git commit -m "Update ${BINARY} to v${VERSION}"
git push

cd -
rm -rf "$TEMP_DIR"

echo ""
echo "Homebrew formula updated to v${VERSION}"
echo "Users can install with: brew install schappim/smack-server/smack-server"
