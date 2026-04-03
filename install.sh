#!/usr/bin/env bash
set -euo pipefail

REPO_OWNER="aiandshubham"
REPO_NAME="gcloud-ai"
INSTALL_DIR=${GAI_INSTALL_DIR:-/usr/local/bin}

# ── Preflight checks ────────────────────────────────────────────────────────

if ! command -v jq &>/dev/null; then
  echo "❌ jq is required but not installed."
  echo "   Install it: brew install jq  (mac) or apt install jq (linux)"
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

# ── Fetch release metadata ───────────────────────────────────────────────────

GAI_VERSION=${GAI_VERSION:-}
if [ "${GAI_VERSION}" ]; then
  echo "🔍 Installing version ${GAI_VERSION} of ${REPO_NAME}..."
  url_tag_path="tags/${GAI_VERSION}"
else
  echo "🔍 Installing latest version of ${REPO_NAME}..."
  url_tag_path="latest"
fi

TMP_DIR=$(mktemp -d)
TMP_RELEASE=$(mktemp)
trap 'rm -rf "$TMP_DIR" "$TMP_RELEASE"' EXIT

# Public repo — no auth needed
curl --silent \
  -H "Accept: application/vnd.github.v3+json" \
  "https://api.github.com/repos/${REPO_OWNER}/${REPO_NAME}/releases/${url_tag_path}" \
  -o "$TMP_RELEASE"

if jq -e '.message' "$TMP_RELEASE" &>/dev/null; then
  echo "❌ GitHub API error: $(jq -r '.message' "$TMP_RELEASE")"
  exit 1
fi

LATEST=$(jq -r '.tag_name' "$TMP_RELEASE")
VERSION="${LATEST#v}"
ASSET="${REPO_NAME}_${VERSION}_${OS}_${ARCH}.tar.gz"

echo "📦 Downloading ${REPO_NAME} ${LATEST} for ${OS}/${ARCH}..."

# ── Parse asset IDs ──────────────────────────────────────────────────────────

ASSET_ID=$(jq -r ".assets[] | select(.name == \"${ASSET}\").id" "$TMP_RELEASE")
CHECKSUM_ID=$(jq -r '.assets[] | select(.name == "checksums.txt").id' "$TMP_RELEASE")

if [ -z "$ASSET_ID" ] || [ "$ASSET_ID" = "null" ]; then
  echo "❌ Asset '${ASSET}' not found in release ${LATEST}."
  echo "   Available assets:"
  jq -r '.assets[].name' "$TMP_RELEASE"
  exit 1
fi

if [ -z "$CHECKSUM_ID" ] || [ "$CHECKSUM_ID" = "null" ]; then
  echo "❌ checksums.txt not found in release ${LATEST}."
  exit 1
fi

# ── Download binary — public repo uses direct download URL ───────────────────

DOWNLOAD_URL=$(jq -r ".assets[] | select(.name == \"${ASSET}\").browser_download_url" "$TMP_RELEASE")
CHECKSUM_URL=$(jq -r '.assets[] | select(.name == "checksums.txt").browser_download_url' "$TMP_RELEASE")

curl --fail --silent --location \
  "$DOWNLOAD_URL" \
  -o "${TMP_DIR}/${ASSET}"

# ── Verify checksum ──────────────────────────────────────────────────────────

echo "🔐 Verifying checksum..."

CHECKSUM_CONTENT=$(curl --fail --silent --location "$CHECKSUM_URL")

EXPECTED=$(echo "$CHECKSUM_CONTENT" | grep "$ASSET" | awk '{print $1}')

if [ -z "$EXPECTED" ]; then
  echo "❌ Could not find checksum for ${ASSET}"
  exit 1
fi

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
  echo "   gcloud-ai requires gcloud for GCP and Kubernetes commands."
fi

echo ""
echo "✅ ${REPO_NAME} ${LATEST} installed successfully!"
echo ""
echo "Next steps:"
echo "  1. Get a free Gemini API key at: https://aistudio.google.com"
echo ""
echo "  2. Set your API key:"
echo "     export GEMINI_API_KEY=your_key"
echo "     echo 'export GEMINI_API_KEY=your_key' >> ~/.zshrc  # make it permanent"
echo ""
echo "  3. Try it:"
echo "     gcloud-ai list all my gcp projects"
echo ""
echo "  Optional — create ~/.gai/config.yml for defaults:"
echo "     gemini_api_key: your_key      # alternative to env var"
echo "     gemini_model: gemini-2.5-pro  # model override"
echo "     default_project: my-project   # skip typing project every time"
echo "     default_region: us-central1   # skip typing region every time"
