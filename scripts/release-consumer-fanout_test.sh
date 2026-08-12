#!/usr/bin/env bash
set -euo pipefail

repo=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
fanout="$repo/scripts/release-consumer-fanout.rb"
manifest="$repo/manifests/release-consumers.yaml"

scratch=$(mktemp -d)
trap 'rm -rf -- "$scratch"' EXIT

expected_repositories='goshtoso,goshtoso-app-shells,goshtoso-charts,manja,paje,xisnove'
test "$("$fanout" repositories "$manifest")" = "$expected_repositories"

metadata="$scratch/release.json"
ruby -rjson - "$metadata" <<'RUBY'
File.write(ARGV.fetch(0), JSON.generate({
  assets_repository: "araihu/assets",
  assets_revision: "a" * 40,
  release: "v1.2.3-rc.1+build.5",
  release_url: "https://github.com/araihu/assets/releases/download/v1.2.3-rc.1+build.5/araihu-assets-v1.2.3-rc.1+build.5.tar.gz",
  release_sha256: "b" * 64,
  release_json_sha256: "c" * 64
}))
RUBY

fake_bin="$scratch/bin"
mkdir -p "$fake_bin"
cat > "$fake_bin/gh" <<'SH'
#!/usr/bin/env bash
set -euo pipefail
endpoint=''
for argument in "$@"; do
  case "$argument" in
    /repos/araihu/*/dispatches) endpoint=$argument ;;
  esac
done
test -n "$endpoint"
repository=${endpoint#/repos/araihu/}
repository=${repository%/dispatches}
printf '%s\n' "$@" > "$GH_LOG_DIR/$repository.args"
cp /dev/stdin "$GH_LOG_DIR/$repository.json"
if [[ "${GH_FAIL_REPOSITORY:-}" == "$repository" ]]; then
  exit 23
fi
SH
chmod +x "$fake_bin/gh"

success_log="$scratch/success"
mkdir -p "$success_log"
PATH="$fake_bin:$PATH" GH_LOG_DIR="$success_log" \
  "$fanout" dispatch "$manifest" "$metadata" > "$scratch/success-summary.md"
retry_log="$scratch/retry"
mkdir -p "$retry_log"
PATH="$fake_bin:$PATH" GH_LOG_DIR="$retry_log" \
  "$fanout" dispatch "$manifest" "$metadata" > "$scratch/retry-summary.md"

IFS=, read -r -a repositories <<< "$expected_repositories"
for repository in "${repositories[@]}"; do
  test -f "$success_log/$repository.args"
  test -f "$success_log/$repository.json"
  test -f "$retry_log/$repository.json"
  grep --fixed-strings --line-regexp --quiet "/repos/araihu/$repository/dispatches" "$success_log/$repository.args"
  ruby -rjson - "$success_log/$repository.json" <<'RUBY'
payload = JSON.parse(File.read(ARGV.fetch(0)))
abort "event type drift" unless payload.keys == ["event_type", "client_payload"] && payload.fetch("event_type") == "araihu-assets-released"
client = payload.fetch("client_payload")
expected = {
  "assets_repository" => "araihu/assets",
  "assets_revision" => "a" * 40,
  "release" => "v1.2.3-rc.1+build.5",
  "release_url" => "https://github.com/araihu/assets/releases/download/v1.2.3-rc.1+build.5/araihu-assets-v1.2.3-rc.1+build.5.tar.gz",
  "release_sha256" => "b" * 64,
  "release_json_sha256" => "c" * 64
}
abort "client payload drift" unless client == expected
RUBY
done

failure_log="$scratch/failure"
mkdir -p "$failure_log"
if PATH="$fake_bin:$PATH" GH_LOG_DIR="$failure_log" GH_FAIL_REPOSITORY=goshtoso-app-shells \
  "$fanout" dispatch "$manifest" "$metadata" > "$scratch/failure-summary.md" 2> "$scratch/failure-error"; then
  echo "fan-out accepted one failed consumer" >&2
  exit 1
fi
for repository in "${repositories[@]}"; do
  test -f "$failure_log/$repository.json"
done
grep --fixed-strings --quiet '`goshtoso-app-shells`: failed' "$scratch/failure-summary.md"
grep --fixed-strings --quiet 'Manual retry: run `Release consumer fan-out`' "$scratch/failure-summary.md"
grep --fixed-strings --quiet 'failed consumers: goshtoso-app-shells' "$scratch/failure-error"

invalid="$scratch/invalid.json"
ruby -rjson - "$metadata" "$invalid" <<'RUBY'
document = JSON.parse(File.read(ARGV.fetch(0)))
document["release"] = "v1"
File.write(ARGV.fetch(1), JSON.generate(document))
RUBY
invalid_log="$scratch/invalid-log"
mkdir -p "$invalid_log"
if PATH="$fake_bin:$PATH" GH_LOG_DIR="$invalid_log" "$fanout" dispatch "$manifest" "$invalid" >/dev/null 2>&1; then
  echo "fan-out accepted a non-SemVer release" >&2
  exit 1
fi
test -z "$(find "$invalid_log" -type f -print -quit)"

echo "release consumer fan-out: payload, order, validation, and aggregate failure passed"
