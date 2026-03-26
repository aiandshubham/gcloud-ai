#!/bin/bash
set -e

REPO_OWNER="Exabeam"
REPO_NAME="gcloud-ai"
INSTALL_DIR="/usr/local/bin"

# ── Preflight checks ────────────────────────────────────────────────────────

if [ -z "$GITHUB_API_TOKEN" ]; then
  echo "❌ GITHUB_API_TOKEN is not set."
  echo "   Export it first: export GITHUB_API_TOKEN=your_token"
  exit 1
fi

if ! command -v curl &>/dev/null; then
  echo "❌ curl is required but not installed."
  exit 1
fi

# ── Detect OS / arch ────────────────────────────────────────────────────────

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

case "$ARCH" in
  x86_64)        ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *)
    echo "❌ Unsupported architecture: $ARCH"
    exit 1
    ;;
esac

# ── Fetch release metadata once ─────────────────────────────────────────────

echo "🔍 Detecting latest release..."

RELEASE_JSON=$(curl -sf \
  -H "Authorization: token ${GITHUB_API_TOKEN}" \
  -H "Accept: application/vnd.github.v3+json" \
  "https://api.github.com/repos/${REPO_OWNER}/${REPO_NAME}/releases/latest")

if [ -z "$RELEASE_JSON" ]; then
  echo "❌ Could not fetch release metadata. Check your token and internet connection."
  exit 1
fi

LATEST=$(echo "$RELEASE_JSON" | grep '"tag_name"' | head -1 | cut -d'"' -f4)

if [ -z "$LATEST" ]; then
  echo "❌ Could not parse tag_name from release."
  exit 1
fi

VERSION="${LATEST#v}"
ASSET="${REPO_NAME}_${VERSION}_${OS}_${ARCH}.tar.gz"

echo "📦 Downloading ${REPO_NAME} ${LATEST} for ${OS}/${ARCH}..."

# ── Parse asset IDs from the single release JSON ────────────────────────────
# GitHub API returns assets as an array; we extract the ID for each asset name.
# The JSON looks like: {"id":12345,"name":"gcloud-ai_1.0.0_darwin_arm64.tar.gz",...}

get_asset_id() {
  local name="$1"
  echo "$RELEASE_JSON" | grep -B1 "\"name\": \"${name}\"" | grep '"id"' | head -1 | grep -o '[0-9]\+'
}

ASSET_ID=$(get_asset_id "$ASSET")
CHECKSUM_ID=$(get_asset_id "checksums.txt")

if [ -z "$ASSET_ID" ]; then
  echo "❌ Asset '${ASSET}' not found in release ${LATEST}."
  echo "   Available assets:"
  echo "$RELEASE_JSON" | grep '"name"' | grep -v 'tag_name\|target' | head -20
  exit 1
fi

if [ -z "$CHECKSUM_ID" ]; then
  echo "❌ checksums.txt not found in release ${LATEST}."
  exit 1
fi

# ── Download binary ──────────────────────────────────────────────────────────

TMP_DIR=$(mktemp -d)
trap 'rm -rf "$TMP_DIR"' EXIT   # always clean up on exit

curl -sfL \
  -H "Authorization: token ${GITHUB_API_TOKEN}" \
  -H "Accept: application/octet-stream" \
  "https://api.github.com/repos/${REPO_OWNER}/${REPO_NAME}/releases/assets/${ASSET_ID}" \
  -o "${TMP_DIR}/${ASSET}"

# ── Verify checksum ──────────────────────────────────────────────────────────

echo "🔐 Verifying checksum..."

CHECKSUM_CONTENT=$(curl -sfL \
  -H "Authorization: token ${GITHUB_API_TOKEN}" \
  -H "Accept: application/octet-stream" \
  "https://api.github.com/repos/${REPO_OWNER}/${REPO_NAME}/releases/assets/${CHECKSUM_ID}")

EXPECTED=$(echo "$CHECKSUM_CONTENT" | grep "$ASSET" | awk '{print $1}')

if [ -z "$EXPECTED" ]; then
  echo "❌ Could not find checksum for ${ASSET} in checksums.txt"
  exit 1
fi

# sha256sum on Linux, shasum on macOS
if command -v sha256sum &>/dev/null; then
  ACTUAL=$(sha256sum "${TMP_DIR}/${ASSET}" | awk '{print $1}')
else
  ACTUAL=$(shasum -a 256 "${TMP_DIR}/${ASSET}" | awk '{print $1}')
fi

if [ "$EXPECTED" != "$ACTUAL" ]; then
  echo "❌ Checksum mismatch — aborting installation"
  echo "   Expected : $EXPECTED"
  echo "   Actual   : $ACTUAL"
  exit 1
fi

# ── Install ──────────────────────────────────────────────────────────────────

echo "📂 Installing to ${INSTALL_DIR}..."
tar -xzf "${TMP_DIR}/${ASSET}" -C "$TMP_DIR"
chmod +x "${TMP_DIR}/${REPO_NAME}"
sudo mv "${TMP_DIR}/${REPO_NAME}" "${INSTALL_DIR}/${REPO_NAME}"

# ── Post-install checks ──────────────────────────────────────────────────────

if ! command -v gcloud &>/dev/null; then
  echo ""
  echo "⚠️  gcloud CLI not found."
  echo "   Install it from: https://cloud.google.com/sdk/docs/install"
  echo "   gcloud-ai requires gcloud to fetch credentials and the Gemini API key."
fi

echo ""
echo "✅ ${REPO_NAME} ${LATEST} installed successfully!"
echo ""
echo "Next steps:"
echo "  1. Log into gcloud:"
echo "     gcloud auth application-default login"
echo ""
echo "  2. Ensure you have Secret Manager access:"
echo "     Project : ops-dist-mgmt"
echo "     Secret  : dev-gemini-key"
echo "     (Ask your admin for roles/secretmanager.secretAccessor if needed)"
echo ""
echo "  3. Persist your token for auto-updates:"
echo "     echo 'export GITHUB_API_TOKEN=your_token' >> ~/.zshrc"
echo ""
echo "  4. Try it:"
echo "     gcloud-ai list all my gcp projects"
