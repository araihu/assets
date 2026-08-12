#!/usr/bin/env bash
set -euo pipefail
repo=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
materialize="$repo/scripts/materialize-dagger-input.sh"
scratch=$(mktemp -d)
trap 'rm -rf -- "$scratch"' EXIT
printf '{"event":"test"}\n' > "$scratch/event.json"

expect_reject() {
  local label=$1
  shift
  if "$@" >/dev/null 2>&1; then
    echo "provider boundary accepted invalid input: $label" >&2
    exit 1
  fi
}

for fork in false true; do
  GITHUB_EVENT_NAME=pull_request GITHUB_EVENT_PATH="$scratch/event.json" GITHUB_HEAD_REPO_FORK="$fork" GITHUB_RUN_ID=123 GITHUB_RUN_ATTEMPT=4 \
    "$materialize" ci "$scratch/ci-$fork.json"
done
ruby -rjson - "$scratch/ci-false.json" "$scratch/ci-true.json" <<'RUBY'
ARGV.each do |path|
  document = JSON.parse(File.read(path))
  abort "internal or fork PR cache namespace differs" unless document == {"cache_namespace"=>"pr", "run_nonce"=>"123-4"}
end
RUBY

for event in push workflow_dispatch; do
  GITHUB_EVENT_NAME="$event" GITHUB_RUN_ID=123 GITHUB_RUN_ATTEMPT=4 \
    "$materialize" ci "$scratch/ci-$event.json"
done
ruby -rjson - "$scratch/ci-push.json" "$scratch/ci-workflow_dispatch.json" <<'RUBY'
ARGV.each do |path|
  abort "protected event cache namespace differs" unless JSON.parse(File.read(path)) == {"cache_namespace"=>"trusted", "run_nonce"=>"123-4"}
end
RUBY

if GITHUB_EVENT_NAME='pull_request_target' GITHUB_RUN_ID=123 GITHUB_RUN_ATTEMPT=4 \
  "$materialize" ci "$scratch/unknown.json" >/dev/null 2>&1; then
  echo 'provider boundary accepted an unknown privileged event' >&2
  exit 1
fi

expect_reject "event name trailing LF" env \
  GITHUB_EVENT_NAME=$'push\n' GITHUB_RUN_ID=123 GITHUB_RUN_ATTEMPT=4 \
  "$materialize" ci "$scratch/reject-event-name.json"
expect_reject "run ID trailing LF" env \
  GITHUB_EVENT_NAME=push GITHUB_RUN_ID=$'123\n' GITHUB_RUN_ATTEMPT=4 \
  "$materialize" ci "$scratch/reject-run-id.json"
expect_reject "run attempt trailing LF" env \
  GITHUB_EVENT_NAME=push GITHUB_RUN_ID=123 GITHUB_RUN_ATTEMPT=$'4\n' \
  "$materialize" ci "$scratch/reject-run-attempt.json"

campaign_plan_env=(
  GITHUB_EVENT_NAME=workflow_dispatch
  GITHUB_REPOSITORY=araihu/assets
  GITHUB_SERVER_URL=https://github.com
  GITHUB_API_URL=https://api.github.com
  GITHUB_RUN_ID=123
  GITHUB_RUN_ATTEMPT=4
  AHAIRU_OWNER=araihu
  AHAIRU_REPOSITORY=goshtoso
  REQUESTED_DATE=2026-01-01
)
expect_reject "campaign event name trailing LF" env "${campaign_plan_env[@]}" \
  GITHUB_EVENT_NAME=$'workflow_dispatch\n' "$materialize" campaign-plan "$scratch/reject-campaign-event.json"
expect_reject "campaign repository trailing LF" env "${campaign_plan_env[@]}" \
  GITHUB_REPOSITORY=$'araihu/assets\n' "$materialize" campaign-plan "$scratch/reject-campaign-repository.json"
expect_reject "campaign owner trailing LF" env "${campaign_plan_env[@]}" \
  AHAIRU_OWNER=$'araihu\n' "$materialize" campaign-plan "$scratch/reject-campaign-owner.json"
expect_reject "campaign target repository trailing LF" env "${campaign_plan_env[@]}" \
  AHAIRU_REPOSITORY=$'goshtoso\n' "$materialize" campaign-plan "$scratch/reject-campaign-target.json"
expect_reject "campaign date trailing LF" env "${campaign_plan_env[@]}" \
  REQUESTED_DATE=$'2026-01-01\n' "$materialize" campaign-plan "$scratch/reject-campaign-date.json"
expect_reject "campaign server URL trailing LF" env "${campaign_plan_env[@]}" \
  GITHUB_SERVER_URL=$'https://github.com\n' "$materialize" campaign-plan "$scratch/reject-campaign-server.json"
expect_reject "campaign API URL trailing LF" env "${campaign_plan_env[@]}" \
  GITHUB_API_URL=$'https://api.github.com\n' "$materialize" campaign-plan "$scratch/reject-campaign-api.json"

injected_owner='araihu; touch /tmp/assets-dagger-input-injection'
GITHUB_EVENT_NAME=workflow_dispatch GITHUB_REPOSITORY=araihu/assets \
GITHUB_SERVER_URL=https://github.com GITHUB_API_URL=https://api.github.com \
GITHUB_RUN_ID=123 GITHUB_RUN_ATTEMPT=4 AHAIRU_OWNER="$injected_owner" \
AHAIRU_REPOSITORY=ahairu REQUESTED_DATE=$'2026-01-01\n$(false)' \
AHAIRU_TOKEN='must-not-be-written' \
  "$materialize" campaign-plan "$scratch/campaign.json"
ruby -rjson - "$scratch/campaign.json" "$injected_owner" <<'RUBY'
document = JSON.parse(File.read(ARGV.fetch(0)))
abort "provider value was not preserved as JSON data" unless document.fetch("ahairu_owner") == ARGV.fetch(1)
abort "multiline value was not preserved as JSON data" unless document.fetch("date") == "2026-01-01\n$(false)"
abort "unexpected payload fields" unless document.keys.sort == %w[ahairu_owner ahairu_repository date github_api_url github_server_url refresh_nonce repository]
abort "secret leaked into payload" if File.read(ARGV.fetch(0)).include?("must-not-be-written")
RUBY
test ! -e /tmp/assets-dagger-input-injection
test "$(stat -c '%a' "$scratch/campaign.json" 2>/dev/null || stat -f '%Lp' "$scratch/campaign.json")" = 600
dispatch_env=(
  GITHUB_EVENT_NAME=workflow_dispatch
  GITHUB_REPOSITORY=araihu/assets
  GITHUB_SERVER_URL=https://github.com
  GITHUB_API_URL=https://api.github.com
  GITHUB_SHA=$(printf 'a%.0s' {1..40})
  GITHUB_RUN_ID=123
  GITHUB_RUN_ATTEMPT=4
  AHAIRU_OWNER=araihu
  AHAIRU_REPOSITORY=goshtoso
  CHANNEL_ARTIFACT_ID=123
  CHANNEL_ARTIFACT_URL=https://github.com/araihu/assets/actions/runs/456/artifacts/123
  CHANNEL_ARTIFACT_SHA256=$(printf 'b%.0s' {1..64})
)
expect_reject "dispatch artifact ID trailing LF" env "${dispatch_env[@]}" \
  CHANNEL_ARTIFACT_ID=$'123\n' "$materialize" campaign-dispatch "$scratch/reject-dispatch-id.json"
expect_reject "dispatch artifact URL trailing LF" env "${dispatch_env[@]}" \
  CHANNEL_ARTIFACT_URL=$'https://github.com/araihu/assets/actions/runs/456/artifacts/123\n' \
  "$materialize" campaign-dispatch "$scratch/reject-dispatch-url.json"
expect_reject "dispatch artifact digest trailing LF" env "${dispatch_env[@]}" \
  CHANNEL_ARTIFACT_SHA256=$'bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb\n' \
  "$materialize" campaign-dispatch "$scratch/reject-dispatch-digest.json"

release_env=(
  GITHUB_EVENT_NAME=push
  GITHUB_REF_TYPE=tag
  GITHUB_REF_NAME=v1.2.3
  GITHUB_SHA=$(printf 'a%.0s' {1..40})
  GITHUB_REPOSITORY=araihu/assets
  GITHUB_RUN_ID=123
  GITHUB_RUN_ATTEMPT=4
)
expect_reject "release tag numeric prerelease leading zero" env "${release_env[@]}" \
  GITHUB_REF_NAME=v1.2.3-01 "$materialize" release-build "$scratch/reject-release-tag.json"
expect_reject "release tag numeric prerelease segment leading zero" env "${release_env[@]}" \
  GITHUB_REF_NAME=v1.2.3-rc.01 "$materialize" release-build "$scratch/reject-release-tag-segment.json"
expect_reject "release tag trailing LF" env "${release_env[@]}" \
  GITHUB_REF_NAME=$'v1.2.3\n' "$materialize" release-build "$scratch/reject-release-tag-lf.json"
expect_reject "release ref type trailing LF" env "${release_env[@]}" \
  GITHUB_REF_TYPE=$'tag\n' "$materialize" release-build "$scratch/reject-release-ref-type.json"
expect_reject "release source revision trailing LF" env "${release_env[@]}" \
  GITHUB_SHA=$'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa\n' "$materialize" release-build "$scratch/reject-release-sha.json"
expect_reject "release repository trailing LF" env "${release_env[@]}" \
  GITHUB_REPOSITORY=$'araihu/assets\n' "$materialize" release-build "$scratch/reject-release-repository.json"
fanout_env=(
  GITHUB_EVENT_NAME=workflow_dispatch
  GITHUB_REPOSITORY=araihu/assets
  GITHUB_RUN_ID=123
  GITHUB_RUN_ATTEMPT=4
  RELEASE=v1.2.3
)
expect_reject "fan-out release trailing LF" env "${fanout_env[@]}" \
  RELEASE=$'v1.2.3\n' "$materialize" fanout-plan "$scratch/reject-fanout-release.json"
expect_reject "fan-out repository trailing LF" env "${fanout_env[@]}" \
  GITHUB_REPOSITORY=$'araihu/assets\n' "$materialize" fanout-plan "$scratch/reject-fanout-repository.json"
touch "$scratch/empty-event.json"
if GITHUB_EVENT_NAME=push GITHUB_EVENT_PATH="$scratch/empty-event.json" GITHUB_RUN_ID=123 GITHUB_RUN_ATTEMPT=4 \
  "$materialize" ci "$scratch/empty.json" >/dev/null 2>&1; then
  echo 'provider boundary accepted an empty event file' >&2
  exit 1
fi

echo 'Dagger provider input: strict JSON transport, cache namespace routing, and injection regression passed'
