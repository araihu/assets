#!/usr/bin/env bash
set -euo pipefail
repo=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
checker="$repo/scripts/check-ci-workflow.sh"

"$checker" "$repo"
"$repo/scripts/materialize-dagger-input_test.sh"
npm --prefix "$repo/.dagger" audit --package-lock-only --omit=dev --audit-level=high

scratch=$(mktemp -d)
trap 'rm -rf -- "$scratch"' EXIT
mutation=0
mutate_and_reject() {
  label=$1 relative=$2 before=$3 after=$4
  mutation=$((mutation + 1))
  fixture="$scratch/mutation-$mutation"
  mkdir -p "$fixture/.github"
  cp -R "$repo/.github/workflows" "$fixture/.github/workflows"
  cp -R "$repo/.dagger" "$fixture/.dagger"
  cp -R "$repo/scripts" "$fixture/scripts"
  cp "$repo/dagger.json" "$repo/go.mod" "$fixture/"
  ruby - "$fixture/$relative" "$before" "$after" <<'RUBY'
path, before, after = ARGV
text = File.read(path)
abort "test fixture marker missing: #{before}" unless text.include?(before)
File.write(path, text.sub(before, after))
RUBY
  if "$checker" "$fixture" >/dev/null 2>&1; then
    echo "CI checker accepted mutation: $label" >&2
    exit 1
  fi
}

mutate_and_reject "pull request cache promoted to trusted" scripts/materialize-dagger-input.sh \
  'pull_request) cache_namespace=pr' 'pull_request) cache_namespace=trusted'
mutate_and_reject "protected cache demoted to PR" scripts/materialize-dagger-input.sh \
  'push|workflow_dispatch) cache_namespace=trusted' 'push|workflow_dispatch) cache_namespace=pr'
mutate_and_reject "unknown event promoted to trusted" scripts/materialize-dagger-input.sh \
  "*) fail 'unsupported CI event' ;;" "*) cache_namespace=trusted ;;"
mutate_and_reject "numeric prerelease leading zero accepted" scripts/materialize-dagger-input.sh \
  '|[1-9][0-9]*|[0-9]*[A-Za-z-][0-9A-Za-z-]*' '|[0-9]+'
mutate_and_reject "provider LF rejection removed" scripts/materialize-dagger-input.sh \
  'must not end with LF' 'provider value accepted'
mutate_and_reject "PR cache namespace hard-coded to trusted" .dagger/src/index.ts \
  'araihu-ci-v1-assets-${cacheNamespace}-go-build-1.26.5' 'araihu-ci-v1-assets-trusted-go-build-1.26.5'
mutate_and_reject "PR runner routed to generic lane" .github/workflows/ci.yml \
  'hostinger-vps-pr' 'hostinger-vps'
mutate_and_reject "protected runner routed to generic lane" .github/workflows/acquisition.yml \
  'hostinger-vps-trusted' 'hostinger-vps'
mutate_and_reject "provider expression enters Dagger args" .github/workflows/ci.yml \
  'version: "0.21.8"' $'version: "0.21.8"\n          args: ci --trust-domain=${{ github.event_name }}'
mutate_and_reject "self-hosted exact version gate removed" .github/workflows/ci.yml \
  'test "$(dagger version | awk '\''NR == 1 { print $2 }'\'')" = v0.21.8' \
  'dagger version'
mutate_and_reject "remote TypeScript SDK dependency restored" .dagger/package.json \
  '"@dagger.io/dagger": "./sdk"' '"@dagger.io/dagger": "0.21.8"'

sdk_fixture="$scratch/versioned-sdk"
mkdir -p "$sdk_fixture/.github" "$sdk_fixture/.dagger/sdk"
cp -R "$repo/.github/workflows" "$sdk_fixture/.github/workflows"
cp -R "$repo/.dagger/." "$sdk_fixture/.dagger/"
cp -R "$repo/scripts" "$sdk_fixture/scripts"
cp "$repo/dagger.json" "$repo/go.mod" "$sdk_fixture/"
printf 'export const fake = true\n' > "$sdk_fixture/.dagger/sdk/index.ts"
git -C "$sdk_fixture" init --quiet
git -C "$sdk_fixture" add --force .dagger/sdk/index.ts
if "$checker" "$sdk_fixture" >/dev/null 2>&1; then
  echo 'CI checker accepted a versioned hand-written SDK fixture' >&2
  exit 1
fi

echo "CI workflow checker: provider/runner/cache/CLI/SDK, anti-fixture, and $mutation mutations passed"
