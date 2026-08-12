#!/usr/bin/env bash
set -euo pipefail
repo=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
materialize="$repo/scripts/materialize-dagger-input.rb"
scratch=$(mktemp -d)
trap 'rm -rf -- "$scratch"' EXIT

for fork in false true; do
  GITHUB_EVENT_NAME=pull_request GITHUB_HEAD_REPO_FORK="$fork" GITHUB_RUN_ID=123 GITHUB_RUN_ATTEMPT=4 \
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

echo 'Dagger provider input: strict JSON transport, cache namespace routing, and injection regression passed'
