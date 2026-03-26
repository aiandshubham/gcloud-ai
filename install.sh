#!/bin/bash
set -e

REPO_OWNER="Exabeam"       # 👈 replace with your GitHub org name
REPO_NAME="gcloud-ai"
INSTALL_DIR="/usr/local/bin"

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
LATEST=$(curl -s "https://api.github.com/repos/${REPO_OWNER}/${REPO_NAME}/releases/latest" \
  | grep '"tag_name"' | cut -d'"' -f4)

if [ -z "$LATEST" ]; then
  echo "❌ Could not fetch latest release. Check your internet connection."
  exit 1
fi

VERSION="${LATEST#v}"
ASSET="${REPO_NAME}_${VERSION}_${OS}_${ARCH}.tar.gz"
DOWNLOAD_URL="https://github.com/${REPO_OWNER}/${REPO_NAME}/releases/download/${LATEST}/${ASSET}"
CHECKSUM_URL="https://github.com/${REPO_OWNER}/${REPO_NAME}/releases/download/${LATEST}/checksums.txt"

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
echo "  3. Set your org name for auto-updates:"
echo "     export GAI_REPO_OWNER=${REPO_OWNER}"
echo "     echo 'export GAI_REPO_OWNER=${REPO_OWNER}' >> ~/.zshrc   # or ~/.bashrc"
echo ""
echo "  4. Try it:"
echo "     gcloud-ai list all my gcp projects"
