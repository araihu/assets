#!/usr/bin/env bash
set -euo pipefail
test -n "$EFFECT_NONCE"
test "$NETWORK_NONCE" = "$EFFECT_NONCE"

ruby -rjson - /plan/plan.json /tmp/dispatch.json <<'RUBY'
plan_path, output = ARGV
plan = JSON.parse(File.read(plan_path))
abort "campaign plan is unchanged" unless plan.fetch("changed")
sha256 = /\A[0-9a-f]{64}\z/
sha1 = /\A[0-9a-f]{40}\z/
artifact_id = Integer(ENV.fetch("CHANNEL_ARTIFACT_ID"), 10)
abort "invalid channel artifact ID" unless artifact_id.positive?
abort "invalid artifact SHA-256" unless ENV.fetch("CHANNEL_ARTIFACT_SHA256").match?(sha256)
abort "invalid bundle digest" unless plan.fetch("bundle_digest").match?(sha256)
abort "invalid source revision" unless ENV.fetch("SOURCE_REVISION").match?(sha1)
expected = %r{\A#{Regexp.escape(ENV.fetch("GITHUB_SERVER_URL"))}/#{Regexp.escape(ENV.fetch("GITHUB_REPOSITORY"))}/actions/runs/[1-9][0-9]*/artifacts/#{artifact_id}\z}
abort "invalid channel artifact URL" unless ENV.fetch("CHANNEL_ARTIFACT_URL").match?(expected)
payload = {
  event_type: "araihu-assets-released",
  client_payload: {
    assets_repository: ENV.fetch("GITHUB_REPOSITORY"),
    assets_revision: ENV.fetch("SOURCE_REVISION"),
    release_artifacts: plan.fetch("release_artifacts"),
    runtime_release: plan.fetch("runtime_release"),
    channel_artifact_id: artifact_id,
    channel_artifact_url: ENV.fetch("CHANNEL_ARTIFACT_URL"),
    channel_artifact_sha256: ENV.fetch("CHANNEL_ARTIFACT_SHA256"),
    candidate_bundle_digest: plan.fetch("bundle_digest"),
    resolution_date: plan.fetch("resolution_date"),
    state: plan.fetch("state")
  }
}
File.write(output, JSON.generate(payload))
RUBY

gh api --method POST \
  -H "Accept: application/vnd.github+json" \
  -H "X-GitHub-Api-Version: 2022-11-28" \
  "/repos/${AHAIRU_OWNER}/${AHAIRU_REPOSITORY}/dispatches" \
  --input /tmp/dispatch.json
echo "campaign dispatch: submitted immutable artifact ${CHANNEL_ARTIFACT_ID}"
