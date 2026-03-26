#!/bin/bash
set -e

REPO_OWNER="Exabeam"
REPO_NAME="gcloud-ai"
INSTALL_DIR="/usr/local/bin"

# Check GITHUB_API_TOKEN is set
if [ -z "$GITHUB_API_TOKEN" ]; then
  echo "❌ GITHUB_API_TOKEN is not set."
  echo "   Export it first: export GITHUB_API_TOKEN=your_token"
  exit 1
fi

# Detect OS and arch
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

case "$ARCH" in
  x86_64)  ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *)
    echo "❌ Unsupported architecture: $ARCH"
    exit 1
    ;;
esac

echo "🔍 Detecting latest release..."
LATEST=$(curl -s "https://${GITHUB_API_TOKEN}@api.github.com/repos/${REPO_OWNER}/${REPO_NAME}/releases/latest" \
  | grep '"tag_name"' | cut -d'"' -f4)

if [ -z "$LATEST" ]; then
  echo "❌ Could not fetch latest release. Check your token and internet connection."
  exit 1
fi

VERSION="${LATEST#v}"
ASSET="${REPO_NAME}_${VERSION}_${OS}_${ARCH}.tar.gz"
DOWNLOAD_URL="https://${GITHUB_API_TOKEN}@github.com/${REPO_OWNER}/${REPO_NAME}/releases/download/${LATEST}/${ASSET}"
CHECKSUM_URL="https://${GITHUB_API_TOKEN}@github.com/${REPO_OWNER}/${REPO_NAME}/releases/download/${LATEST}/checksums.txt"

echo "📦 Downloading ${REPO_NAME} ${LATEST} for ${OS}/${ARCH}..."
TMP_DIR=$(mktemp -d)
curl -sL "$DOWNLOAD_URL" -o "${TMP_DIR}/${ASSET}"

echo "🔐 Verifying checksum..."
EXPECTED=$(curl -sL "$CHECKSUM_URL" | grep "$ASSET" | awk '{print $1}')
ACTUAL=$(sha256sum "${TMP_DIR}/${ASSET}" 2>/dev/null || shasum -a 256 "${TMP_DIR}/${ASSET}" | awk '{print $1}')

if [ "$EXPECTED" != "$ACTUAL" ]; then
  echo "❌ Checksum mismatch — aborting installation"
  rm -rf "$TMP_DIR"
  exit 1
fi

echo "📂 Installing to ${INSTALL_DIR}..."
tar -xzf "${TMP_DIR}/${ASSET}" -C "$TMP_DIR"
chmod +x "${TMP_DIR}/${REPO_NAME}"
sudo mv "${TMP_DIR}/${REPO_NAME}" "${INSTALL_DIR}/${REPO_NAME}"
rm -rf "$TMP_DIR"

# Verify gcloud is available
if ! command -v gcloud &>/dev/null; then
  echo ""
  echo "⚠️  gcloud CLI not found."
  echo "   Install it from: https://cloud.google.com/sdk/docs/install"
  echo "   gcloud-ai requires gcloud to fetch credentials and the API key."
fi

echo ""
echo "✅ ${REPO_NAME} ${LATEST} installed successfully!"
echo ""
echo "Next steps:"
echo "  1. Make sure you are logged into gcloud:"
echo "     gcloud auth application-default login"
echo ""
echo "  2. Make sure you have access to the secret in GCP:"
echo "     Project : ops-dist-mgmt"
echo "     Secret  : dev-gemini-key"
echo "     (Ask your admin to grant you roles/secretmanager.secretAccessor if needed)"
echo ""
echo "  3. Set your GitHub token for auto-updates:"
echo "     export GITHUB_API_TOKEN=your_token"
echo "     echo 'export GITHUB_API_TOKEN=your_token' >> ~/.zshrc   # or ~/.bashrc"
echo ""
echo "  4. Try it:"
echo "     gcloud-ai list all my gcp projects"
