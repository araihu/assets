#!/usr/bin/env bash
set -euo pipefail

repo=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
checker="$repo/scripts/check-release-workflows.sh"

"$checker" "$repo"

fixture=$(mktemp -d)
trap 'rm -rf -- "$fixture"' EXIT
mkdir -p "$fixture/.github/workflows"
cp "$repo/.github/workflows/release.yml" "$fixture/.github/workflows/release.yml"
cp "$repo/.github/workflows/campaigns.yml" "$fixture/.github/workflows/campaigns.yml"
cp "$repo/.github/workflows/ci.yml" "$fixture/.github/workflows/ci.yml"

ruby - "$fixture/.github/workflows/campaigns.yml" <<'RUBY'
path = ARGV.fetch(0)
text = File.read(path)
expected = 'cron: "0 0 * * *"'
abort "test fixture: schedule marker missing" unless text.include?(expected)
File.write(path, text.sub(expected, 'cron: "5 0 * * *"'))
RUBY

if "$checker" "$fixture" >/dev/null 2>&1; then
  echo "workflow checker accepted a non-midnight campaign schedule" >&2
  exit 1
fi

echo "release workflow checker: mutation rejected"
