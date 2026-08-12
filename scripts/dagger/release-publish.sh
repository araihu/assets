#!/usr/bin/env bash
set -euo pipefail
test -n "$EFFECT_NONCE"
test "$NETWORK_NONCE" = "$EFFECT_NONCE"

bundle=/release-output/release-bundle
assets=(
  "$bundle/araihu-assets-${RELEASE_TAG}.tar.gz"
  "$bundle/araihu-assets-${RELEASE_TAG}.zip"
  "$bundle/release.json"
  "$bundle/latest.json"
  "$bundle/checksums.txt"
  "$bundle/SHA256SUMS"
)
for asset in "${assets[@]}"; do test -f "$asset"; done
(cd "$bundle" && sha256sum --check --strict SHA256SUMS)

if gh release view "$RELEASE_TAG" --repo "$GITHUB_REPOSITORY" >/dev/null 2>&1; then
  mkdir -p /tmp/existing-release
  gh release view "$RELEASE_TAG" --repo "$GITHUB_REPOSITORY" --json assets --jq '.assets[].name' > /tmp/existing-release-assets
  missing=()
  for asset in "${assets[@]}"; do
    name=$(basename "$asset")
    if grep --fixed-strings --line-regexp --quiet "$name" /tmp/existing-release-assets; then
      gh release download "$RELEASE_TAG" --repo "$GITHUB_REPOSITORY" --pattern "$name" --dir /tmp/existing-release
      test "$(sha256sum "$asset" | awk '{print $1}')" = "$(sha256sum "/tmp/existing-release/$name" | awk '{print $1}')" || {
        echo "existing release asset differs: $name" >&2
        exit 1
      }
    else
      missing+=("$asset")
    fi
  done
  for asset in "${missing[@]}"; do
    gh release upload "$RELEASE_TAG" "$asset" --repo "$GITHUB_REPOSITORY"
  done
else
  gh release create "$RELEASE_TAG" "${assets[@]}" --repo "$GITHUB_REPOSITORY" \
    --verify-tag --title "$RELEASE_TAG" --generate-notes
fi
echo "release publication: ${RELEASE_TAG} verified or completed"
