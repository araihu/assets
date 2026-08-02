#!/usr/bin/env bash
set -euo pipefail

repo=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
checker="$repo/scripts/check-release-workflows.sh"

"$checker" "$repo"
"$repo/scripts/release-consumer-fanout_test.sh"

scratch=$(mktemp -d)
trap 'rm -rf -- "$scratch"' EXIT

members="$scratch/archive-members"
printf 'release.json\ncampaign/v1.js\n' > "$members"
ruby "$repo/scripts/validate-release-archive-members.rb" "$members"
for invalid in exact casefold unicode traversal; do
  case "$invalid" in
    exact) printf 'release.json\nrelease.json\n' > "$members" ;;
    casefold) printf 'Release.json\nrelease.json\n' > "$members" ;;
    unicode)
      ruby - "$members" <<'RUBY'
File.binwrite(ARGV.fetch(0), "caf\u00e9.json\ncafe\u0301.json\n")
RUBY
      ;;
    traversal) printf '../release.json\n' > "$members" ;;
  esac
  if ruby "$repo/scripts/validate-release-archive-members.rb" "$members" >/dev/null 2>&1; then
    echo "archive member validator accepted $invalid collision" >&2
    exit 1
  fi
done

bundle="$scratch/bundle"
mkdir -p "$bundle/campaign" "$bundle/releases"
printf 'runtime-a\n' > "$bundle/campaign/v1.js"
printf 'latest-a\n' > "$bundle/releases/latest.json"
printf 'default-a\n' > "$bundle/releases/default.json"
printf 'current-a\n' > "$bundle/releases/current.json"
digest_a=$("$repo/scripts/channel-bundle-digest.rb" "$bundle")
printf 'latest-b\n' > "$bundle/releases/latest.json"
digest_latest=$("$repo/scripts/channel-bundle-digest.rb" "$bundle")
test "$digest_a" != "$digest_latest"
printf 'latest-a\n' > "$bundle/releases/latest.json"
printf 'runtime-b\n' > "$bundle/campaign/v1.js"
digest_runtime=$("$repo/scripts/channel-bundle-digest.rb" "$bundle")
test "$digest_a" != "$digest_runtime"
printf 'unexpected\n' > "$bundle/extra"
if "$repo/scripts/channel-bundle-digest.rb" "$bundle" >/dev/null 2>&1; then
  echo "bundle digest accepted an unexpected fifth path" >&2
  exit 1
fi

state_response="$scratch/accepted-state.json"
ruby -rbase64 -rjson - "$state_response" "$digest_latest" <<'RUBY'
path, digest = ARGV
record = {
  schemaVersion: 1,
  bundleDigest: digest,
  channelArtifactId: 123,
  channelArtifactUrl: "https://github.com/araihu/assets/actions/runs/456/artifacts/123",
  channelArtifactSha256: "a" * 64,
  sourceRepository: "araihu/assets",
  sourceWorkflow: "araihu/assets/.github/workflows/campaigns.yml",
  sourceRevision: "b" * 40,
  release: "v0.1.0",
  releaseUrl: "https://github.com/araihu/assets/releases/download/v0.1.0/araihu-assets-v0.1.0.tar.gz",
  releaseSha256: "c" * 64,
  resolutionDate: "2026-10-31"
}
wrapper = {
  type: "file",
  path: ".automation/araihu-assets/accepted-channel-v1.json",
  encoding: "base64",
  sha: "d" * 40,
  content: Base64.strict_encode64(JSON.generate(record))
}
File.write(path, JSON.generate(wrapper))
RUBY
decision_b=$("$repo/scripts/accepted-channel-state.rb" "$state_response" "$digest_latest" \
  araihu/assets araihu/assets/.github/workflows/campaigns.yml https://github.com \
  .automation/araihu-assets/accepted-channel-v1.json)
test "$(ruby -rjson -e 'puts JSON.parse(ARGV.fetch(0)).fetch("changed")' "$decision_b")" = false
decision_a_again=$("$repo/scripts/accepted-channel-state.rb" "$state_response" "$digest_a" \
  araihu/assets araihu/assets/.github/workflows/campaigns.yml https://github.com \
  .automation/araihu-assets/accepted-channel-v1.json)
test "$(ruby -rjson -e 'puts JSON.parse(ARGV.fetch(0)).fetch("changed")' "$decision_a_again")" = true

mutation=0
mutate_and_reject() {
  label=$1
  relative=$2
  before=$3
  after=$4
  mutation=$((mutation + 1))
  fixture="$scratch/mutation-$mutation"
  mkdir -p "$fixture/.github/workflows" "$fixture/scripts" "$fixture/manifests"
  cp "$repo/.github/workflows/release.yml" "$fixture/.github/workflows/release.yml"
  cp "$repo/.github/workflows/release-fanout.yml" "$fixture/.github/workflows/release-fanout.yml"
  cp "$repo/.github/workflows/campaigns.yml" "$fixture/.github/workflows/campaigns.yml"
  cp "$repo/.github/workflows/ci.yml" "$fixture/.github/workflows/ci.yml"
  cp "$repo/scripts/channel-bundle-digest.rb" "$fixture/scripts/channel-bundle-digest.rb"
  cp "$repo/scripts/validate-release-archive-members.rb" "$fixture/scripts/validate-release-archive-members.rb"
  cp "$repo/scripts/accepted-channel-state.rb" "$fixture/scripts/accepted-channel-state.rb"
  cp "$repo/scripts/release-consumer-fanout.rb" "$fixture/scripts/release-consumer-fanout.rb"
  cp "$repo/manifests/release-consumers.yaml" "$fixture/manifests/release-consumers.yaml"
  ruby - "$fixture/$relative" "$before" "$after" <<'RUBY'
path, before, after = ARGV
text = File.read(path)
abort "test fixture marker missing: #{before}" unless text.include?(before)
File.write(path, text.sub(before, after))
RUBY
  if "$checker" "$fixture" >/dev/null 2>&1; then
    echo "workflow checker accepted mutation: $label" >&2
    exit 1
  fi
}

mutate_and_reject "mutable release checkout" .github/workflows/release.yml \
  'ref: ${{ github.sha }}' 'ref: ${{ github.ref }}'
mutate_and_reject "missing tag-to-event SHA verification" .github/workflows/release.yml \
  'test "$tag_sha" = "$event_sha"' 'test -n "$tag_sha"'
mutate_and_reject "missing pre-publication App credential gate" .github/workflows/release.yml \
  'id: fanout-credentials' 'id: fanout-credentials-disabled'
mutate_and_reject "pre-publication gate ignores private key" .github/workflows/release.yml \
  '[[ -z "$ARAIHU_ASSETS_APP_PRIVATE_KEY" ]]' '[[ -n "$ARAIHU_ASSETS_APP_PRIVATE_KEY" ]]'
mutate_and_reject "upload during release preflight" .github/workflows/release.yml \
  'missing+=("$asset")' 'gh release upload "$TAG" "$asset" --repo "$GITHUB_REPOSITORY"'
mutate_and_reject "clobbering release upload" .github/workflows/release.yml \
  'gh release upload "$TAG" "$asset" --repo "$GITHUB_REPOSITORY"' 'gh release upload "$TAG" "$asset" --repo "$GITHUB_REPOSITORY" --clobber'
mutate_and_reject "release metadata renamed as latest channel" .github/workflows/release.yml \
  'go run ./cmd/araihu-assets campaigns publish --date "$CHANNEL_DATE" --output "$channel_candidate"' 'install -m 0644 dist/release.json "$latest/latest.json"'
mutate_and_reject "release uses untagged dist as promoted snapshot" .github/workflows/release.yml \
  'gh release download "$default_release"' 'cp -R dist "$default_root"'
mutate_and_reject "release omits archive collision validation" .github/workflows/release.yml \
  'ruby scripts/validate-release-archive-members.rb "$default_download/archive.members"' 'test -s "$default_download/archive.members"'
mutate_and_reject "release omits fallback consumer fan-out" .github/workflows/release.yml \
  'uses: ./.github/workflows/release-fanout.yml' 'uses: ./.github/workflows/missing-fanout.yml'
mutate_and_reject "manual fan-out disabled" .github/workflows/release-fanout.yml \
  'workflow_dispatch:' 'disabled_workflow_dispatch:'
mutate_and_reject "fan-out token targets every installation repository" .github/workflows/release-fanout.yml \
  'repositories: ${{ steps.consumers.outputs.repositories }}' 'repositories: ""'
mutate_and_reject "fan-out token minted before release verification" .github/workflows/release-fanout.yml \
  'id: consumers' 'id: consumers-disabled'
mutate_and_reject "fallback enrollment includes metaru" manifests/release-consumers.yaml \
  '  - xisnove' '  - metaru'
mutate_and_reject "fallback dispatch event type drift" scripts/release-consumer-fanout.rb \
  '"event_type" => "araihu-assets-released"' '"event_type" => "araihu-assets-current"'
mutate_and_reject "untagged promoted release shortcut" .github/workflows/campaigns.yml \
  'materialize_release "$default_release"' 'stage_snapshot "$default_release" "dist"'
mutate_and_reject "untagged runtime handoff" .github/workflows/campaigns.yml \
  'cmp --silent "$bundle/campaign/v1.js" "releases/${{ steps.release.outputs.default_release }}/campaign/v1.js"' 'cmp --silent "$bundle/campaign/v1.js" "dist/campaign/v1.js"'
mutate_and_reject "missing release archive hash check" .github/workflows/campaigns.yml \
  'sha256sum --check --strict "$download/archive.sha256"' 'test -f "$download/archive.sha256"'
mutate_and_reject "campaign omits archive collision validation" .github/workflows/campaigns.yml \
  'ruby scripts/validate-release-archive-members.rb "$download/archive.members"' 'test -s "$download/archive.members"'
mutate_and_reject "missing published latest hash check" .github/workflows/campaigns.yml \
  'sha256sum --check --strict "$latest_download/latest.sha256"' 'test -f "$latest_download/latest.sha256"'
mutate_and_reject "historical accepted artifact lookup" .github/workflows/campaigns.yml \
  'contents/${STATE_PATH}?ref=${STATE_REF}' 'actions/artifacts?name=accepted-channel-${bundle_digest}'
mutate_and_reject "retention-backed accepted state" .github/workflows/campaigns.yml \
  'name: channel-${{ steps.channel.outputs.digest }}' 'name: accepted-channel-${{ steps.channel.outputs.digest }}'
mutate_and_reject "inverted current-state comparison" scripts/accepted-channel-state.rb \
  'accepted_digest != bundle_digest' 'accepted_digest == bundle_digest'
mutate_and_reject "Assets accepts before Ahairu deployment" .github/workflows/campaigns.yml \
  '            --input "$payload"' '            --input "$payload"'$'\n''          curl --request PUT "${GITHUB_API_URL}/repos/${AHAIRU_OWNER}/${AHAIRU_REPOSITORY}/contents/${STATE_PATH}"'
mutate_and_reject "dispatch omits distinct release identities" .github/workflows/campaigns.yml \
  'release_artifacts: releases,' 'release: runtime_release,'
mutate_and_reject "dispatch omits candidate digest" .github/workflows/campaigns.yml \
  'candidate_bundle_digest: bundle_digest,' 'bundle_digest: nil,'
mutate_and_reject "dispatch renames compact accepted-state envelope" .github/workflows/campaigns.yml \
  '                state: {' '                accepted_state: {'
mutate_and_reject "dispatch omits accepted-state ref" .github/workflows/campaigns.yml \
  'ref: ENV.fetch("STATE_REF"),' 'ref: nil,'
mutate_and_reject "dispatch omits accepted-state path" .github/workflows/campaigns.yml \
  'path: ENV.fetch("STATE_PATH")' 'path: nil'

echo "release workflow checker: canonical digest and $mutation mutations passed"
