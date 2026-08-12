#!/usr/bin/env bash
set -euo pipefail
repo=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
checker="$repo/scripts/check-release-workflows.sh"

"$checker" "$repo"
"$repo/scripts/release-consumer-fanout_test.sh"
for script in "$repo"/scripts/dagger/*.sh; do bash -n "$script"; done

scratch=$(mktemp -d)
trap 'rm -rf -- "$scratch"' EXIT

members="$scratch/archive-members"
printf 'release.json\ncampaign/v1.js\n' > "$members"
ruby "$repo/scripts/validate-release-archive-members.rb" "$members"
for invalid in exact casefold unicode traversal; do
  case "$invalid" in
    exact) printf 'release.json\nrelease.json\n' > "$members" ;;
    casefold) printf 'Release.json\nrelease.json\n' > "$members" ;;
    unicode) ruby - "$members" <<'RUBY'
File.binwrite(ARGV.fetch(0), "caf\u00e9.json\ncafe\u0301.json\n")
RUBY
      ;;
    traversal) printf '../release.json\n' > "$members" ;;
  esac
  if ruby "$repo/scripts/validate-release-archive-members.rb" "$members" >/dev/null 2>&1; then
    echo "archive member validator accepted $invalid" >&2
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
digest_b=$("$repo/scripts/channel-bundle-digest.rb" "$bundle")
test "$digest_a" != "$digest_b"
printf 'unexpected\n' > "$bundle/extra"
if "$repo/scripts/channel-bundle-digest.rb" "$bundle" >/dev/null 2>&1; then
  echo 'bundle digest accepted an unexpected path' >&2
  exit 1
fi

state_response="$scratch/accepted-state.json"
ruby -rbase64 -rjson - "$state_response" "$digest_b" <<'RUBY'
path, digest = ARGV
record = {
  schemaVersion: 1, bundleDigest: digest, channelArtifactId: 123,
  channelArtifactUrl: "https://github.com/araihu/assets/actions/runs/456/artifacts/123",
  channelArtifactSha256: "a" * 64, sourceRepository: "araihu/assets",
  sourceWorkflow: "araihu/assets/.github/workflows/campaigns.yml", sourceRevision: "b" * 40,
  release: "v0.1.0", releaseUrl: "https://github.com/araihu/assets/releases/download/v0.1.0/araihu-assets-v0.1.0.tar.gz",
  releaseSha256: "c" * 64, resolutionDate: "2026-10-31"
}
wrapper = {type: "file", path: ".automation/araihu-assets/accepted-channel-v1.json", encoding: "base64", sha: "d" * 40, content: Base64.strict_encode64(JSON.generate(record))}
File.write(path, JSON.generate(wrapper))
RUBY
decision=$("$repo/scripts/accepted-channel-state.rb" "$state_response" "$digest_b" araihu/assets \
  araihu/assets/.github/workflows/campaigns.yml https://github.com .automation/araihu-assets/accepted-channel-v1.json)
test "$(ruby -rjson -e 'puts JSON.parse(ARGV.fetch(0)).fetch("changed")' "$decision")" = false

mutation=0
mutate_and_reject() {
  label=$1 relative=$2 before=$3 after=$4
  mutation=$((mutation + 1))
  fixture="$scratch/mutation-$mutation"
  mkdir -p "$fixture/.github" "$fixture"
  cp -R "$repo/.github/workflows" "$fixture/.github/workflows"
  cp -R "$repo/.dagger" "$fixture/.dagger"
  cp -R "$repo/scripts" "$fixture/scripts"
  cp -R "$repo/manifests" "$fixture/manifests"
  ruby - "$fixture/$relative" "$before" "$after" <<'RUBY'
path, before, after = ARGV
text = File.read(path)
abort "test fixture marker missing: #{before}" unless text.include?(before)
File.write(path, text.sub(before, after))
RUBY
  if "$checker" "$fixture" >/dev/null 2>&1; then
    echo "release checker accepted mutation: $label" >&2
    exit 1
  fi
}

mutate_and_reject "tag is not bound to event commit" .github/workflows/release.yml \
  'test "$tag_sha" = "$event_sha"' 'test -n "$tag_sha"'
mutate_and_reject "credential preflight removed" .github/workflows/release.yml \
  'id: fanout-credentials' 'id: fanout-credentials-disabled'
mutate_and_reject "private key preflight inverted" .github/workflows/release.yml \
  '[[ -z "$ARAIHU_ASSETS_APP_PRIVATE_KEY" ]]' '[[ -n "$ARAIHU_ASSETS_APP_PRIVATE_KEY" ]]'
mutate_and_reject "release upload clobbers published bytes" scripts/dagger/release-publish.sh \
  'gh release upload "$RELEASE_TAG" "$asset" --repo "$GITHUB_REPOSITORY"' \
  'gh release upload "$RELEASE_TAG" "$asset" --repo "$GITHUB_REPOSITORY" --clobber'
mutate_and_reject "release skips archive collision validation" scripts/dagger/release-build.sh \
  'ruby scripts/validate-release-archive-members.rb "$default_download/archive.members"' \
  'test -s "$default_download/archive.members"'
mutate_and_reject "fallback fan-out removed" .github/workflows/release.yml \
  'uses: ./.github/workflows/release-fanout.yml' 'uses: ./.github/workflows/missing-fanout.yml'
mutate_and_reject "manual fan-out retry removed" .github/workflows/release-fanout.yml \
  'workflow_dispatch:' 'disabled_workflow_dispatch:'
mutate_and_reject "fan-out token is not consumer-scoped" .github/workflows/release-fanout.yml \
  'repositories: ${{ steps.consumers.outputs.repositories }}' 'repositories: ""'
mutate_and_reject "fan-out event type drifts" scripts/release-consumer-fanout.rb \
  '"event_type" => "araihu-assets-released"' '"event_type" => "araihu-assets-current"'
mutate_and_reject "fan-out stops aggregating all consumers" scripts/release-consumer-fanout.rb \
  'results = repositories.map do |repository|' 'results = repositories.take(1).map do |repository|'
mutate_and_reject "fallback enrollment drifts" manifests/release-consumers.yaml \
  '  - xisnove' '  - metaru'
mutate_and_reject "accepted digest comparison inverted" scripts/accepted-channel-state.rb \
  'accepted_digest != bundle_digest' 'accepted_digest == bundle_digest'
mutate_and_reject "accepted repository provenance omitted" scripts/accepted-channel-state.rb \
  'state.fetch("sourceRepository") == source_repository' 'state.fetch("sourceRepository").is_a?(String)'
mutate_and_reject "accepted workflow provenance omitted" scripts/accepted-channel-state.rb \
  'state.fetch("sourceWorkflow") == source_workflow' 'state.fetch("sourceWorkflow").is_a?(String)'
mutate_and_reject "campaign skips archive collision validation" scripts/dagger/campaign-plan.sh \
  'ruby scripts/validate-release-archive-members.rb "$download/archive.members"' \
  'test -s "$download/archive.members"'
mutate_and_reject "campaign skips published latest hash" scripts/dagger/campaign-plan.sh \
  'sha256sum --check --strict "$latest_download/latest.sha256"' 'test -f "$latest_download/latest.sha256"'
mutate_and_reject "campaign provider output path drifts" .github/workflows/campaigns.yml \
  'provider_output="${RUNNER_TEMP}/campaign-plan/provider-output"' 'provider_output="${RUNNER_TEMP}/campaign-plan/plan.json"'
mutate_and_reject "host node runtime restored" .github/workflows/campaigns.yml \
  '      - name: Expose campaign plan to provider boundary' $'      - name: Host node\n        run: node --version\n\n      - name: Expose campaign plan to provider boundary'
mutate_and_reject "release materializer permits numeric prerelease leading zero" scripts/materialize-dagger-input.sh \
  '|[1-9][0-9]*|[0-9]*[A-Za-z-][0-9A-Za-z-]*' '|[0-9]+'
mutate_and_reject "campaign provider digest is not strict" .github/workflows/campaigns.yml \
  '[[ "$digest" =~ ^[0-9a-f]{64}$ ]]' 'test -n "$digest"'
mutate_and_reject "campaign Dagger output omits validated lines" scripts/dagger/campaign-plan.sh \
  "printf 'changed=%s\\ndigest=%s\\n' \"\$changed\" \"\$bundle_digest\" > /out/provider-output" \
  "printf 'changed=%s\\n' \"\$changed\" > /out/provider-output"

echo "release workflow checker: runtime archive/state/fan-out and $mutation mutations passed"
