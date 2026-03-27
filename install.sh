#!/usr/bin/env bash
set -euo pipefail

REPO_OWNER="Exabeam"
REPO_NAME="gcloud-ai"
INSTALL_DIR=${GAI_INSTALL_DIR:-/usr/local/bin}

# ── Preflight checks ────────────────────────────────────────────────────────

if ! command -v jq &>/dev/null; then
  echo "❌ jq is required but not installed."
  echo "   Install it: brew install jq  (mac) or apt install jq (linux)"
  exit 1
fi

GITHUB_API_TOKEN=${GITHUB_API_TOKEN:-}
if [ -z "${GITHUB_API_TOKEN}" ]; then
  echo "❌ GITHUB_API_TOKEN must be set in the environment."
  echo "   Export it first: export GITHUB_API_TOKEN=your_token"
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

curl --silent \
  -H "Authorization: token ${GITHUB_API_TOKEN}" \
  -H "Accept: application/vnd.github.v3+json" \
  "https://api.github.com/repos/${REPO_OWNER}/${REPO_NAME}/releases/${url_tag_path}" \
  -o "$TMP_RELEASE"

# Check for API error
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

# ── Download binary ──────────────────────────────────────────────────────────

curl --fail --silent --location \
  -H "Accept: application/octet-stream" \
  "https://${GITHUB_API_TOKEN}@api.github.com/repos/${REPO_OWNER}/${REPO_NAME}/releases/assets/${ASSET_ID}" \
  -o "${TMP_DIR}/${ASSET}"

# ── Verify checksum ──────────────────────────────────────────────────────────

echo "🔐 Verifying checksum..."

CHECKSUM_CONTENT=$(curl --fail --silent --location \
  -H "Accept: application/octet-stream" \
  "https://${GITHUB_API_TOKEN}@api.github.com/repos/${REPO_OWNER}/${REPO_NAME}/releases/assets/${CHECKSUM_ID}")

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
  echo "   gcloud-ai requires gcloud to fetch credentials and the Gemini API key."
fi

echo ""
echo "✅ ${REPO_NAME} ${LATEST} installed successfully!"
echo ""
echo "Next steps:"
echo "  1. Run following bootstrap commands to begin:"
echo "     gcloud auth login"                                                                           
echo "     gcloud auth application-default login"
echo "     gcloud components install gke-gcloud-auth-plugin -q"
echo "     export USE_GKE_GCLOUD_AUTH_PLUGIN=True"
echo "     ecp kubeconfig"
echo ""
echo "  2. Try it:"
echo "     gcloud-ai list all my gcp projects"
