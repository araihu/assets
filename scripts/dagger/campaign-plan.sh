#!/usr/bin/env bash
set -euo pipefail
test -n "$NETWORK_NONCE"

release_pattern='\Av(?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*)(?:-[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?(?:\+[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?\z'
if [[ -n "$REQUESTED_DATE" ]]; then
  resolution_date=$(REQUESTED_DATE="$REQUESTED_DATE" ruby -rdate -e '
    raw = ENV.fetch("REQUESTED_DATE")
    parsed = Date.iso8601(raw)
    abort "date must use YYYY-MM-DD" unless parsed.iso8601 == raw
    puts raw
  ')
else
  resolution_date=$(date -u +%F)
fi

default_release=$(ruby -ryaml -e 'puts YAML.safe_load(File.read("manifests/default.yaml")).fetch("release")')
latest_release=$(gh release view --repo "$GITHUB_REPOSITORY" --json tagName --jq .tagName)
ruby -e 'pattern = Regexp.new(ARGV.shift); ARGV.each { |value| abort "invalid release tag" unless value.match?(pattern) }' \
  "$release_pattern" "$default_release" "$latest_release"

published_root=/tmp/published-releases
records=/tmp/release-artifacts.jsonl
mkdir -p "$published_root"
: > "$records"

materialize_release() {
  local release=$1 archive_name download extract release_sha256 release_url
  archive_name="araihu-assets-${release}.tar.gz"
  download="$published_root/$release/download"
  extract="$published_root/$release/root"
  mkdir -p "$download" "$extract"
  gh release download "$release" --repo "$GITHUB_REPOSITORY" --pattern "$archive_name" --dir "$download"
  gh release download "$release" --repo "$GITHUB_REPOSITORY" --pattern SHA256SUMS --dir "$download"
  release_sha256=$(ruby -e '
    name, path = ARGV
    matches = File.readlines(path, chomp: true).filter_map { |line| hash, candidate = line.split(/\s+/, 2); hash if candidate == name }
    abort "release archive hash must appear exactly once" unless matches.length == 1 && matches.first.match?(/\A[0-9a-f]{64}\z/)
    puts matches.first
  ' "$archive_name" "$download/SHA256SUMS")
  printf '%s  %s\n' "$release_sha256" "$download/$archive_name" > "$download/archive.sha256"
  sha256sum --check --strict "$download/archive.sha256"
  tar --list --gzip --file "$download/$archive_name" > "$download/archive.members"
  ruby scripts/validate-release-archive-members.rb "$download/archive.members"
  tar --list --verbose --gzip --file "$download/$archive_name" | awk 'substr($1, 1, 1) != "-" { exit 1 }'
  tar --extract --gzip --file "$download/$archive_name" --directory "$extract"
  (cd "$extract" && sha256sum --check --strict checksums.txt)
  for contract in release.json catalog.json themes.json campaigns.json campaign/v1.js; do
    test -f "$extract/$contract"
    test ! -L "$extract/$contract"
  done
  ruby -rjson -e 'document = JSON.parse(File.read(ARGV[0])); abort "release metadata mismatch" unless document.fetch("release") == ARGV[1]' \
    "$extract/release.json" "$release"
  release_url="${GITHUB_SERVER_URL}/${GITHUB_REPOSITORY}/releases/download/${release}/${archive_name}"
  ruby -rjson -e 'puts JSON.generate({release: ARGV[0], release_url: ARGV[1], release_sha256: ARGV[2]})' \
    "$release" "$release_url" "$release_sha256" >> "$records"
}

stage_snapshot() {
  local release=$1 target=$2 source="$published_root/$1/root"
  mkdir -p "$target/campaign"
  for contract in release.json catalog.json themes.json campaigns.json; do
    install -m 0644 "$source/$contract" "$target/$contract"
  done
  install -m 0644 "$source/campaign/v1.js" "$target/campaign/v1.js"
}

materialize_release "$default_release"
if [[ "$latest_release" != "$default_release" ]]; then materialize_release "$latest_release"; fi
stage_snapshot "$default_release" "releases/${default_release}"
stage_snapshot "$latest_release" releases/latest

latest_download="$published_root/$latest_release/download"
gh release download "$latest_release" --repo "$GITHUB_REPOSITORY" --pattern latest.json --dir "$latest_download"
latest_sha256=$(ruby -e '
  matches = File.readlines(ARGV[0], chomp: true).filter_map { |line| hash, candidate = line.split(/\s+/, 2); hash if candidate == "latest.json" }
  abort "latest.json hash must appear exactly once" unless matches.length == 1 && matches.first.match?(/\A[0-9a-f]{64}\z/)
  puts matches.first
' "$latest_download/SHA256SUMS")
printf '%s  %s\n' "$latest_sha256" "$latest_download/latest.json" > "$latest_download/latest.sha256"
sha256sum --check --strict "$latest_download/latest.sha256"
install -m 0644 "$latest_download/latest.json" releases/latest/latest.json

release_artifacts=$(ruby -rjson -e '
  records = File.readlines(ARGV[0], chomp: true).map { |line| JSON.parse(line) }
  abort "duplicate release artifact" unless records.map { |record| record.fetch("release") }.uniq.length == records.length
  print JSON.generate(records.sort_by { |record| record.fetch("release") })
' "$records")

test "$(go version | awk '{print $3}')" = go1.27.0
go run ./cmd/araihu-assets themes validate
go run ./cmd/araihu-assets campaigns validate
mkdir -p /out/bundle
go run ./cmd/araihu-assets campaigns publish --date "$resolution_date" --output /out/bundle
cmp --silent /out/bundle/releases/latest.json releases/latest/latest.json
cmp --silent /out/bundle/campaign/v1.js "releases/${default_release}/campaign/v1.js"
RELEASE_ARTIFACTS="$release_artifacts" ruby -rjson -e '
  documents = %w[latest default current].map { |name| JSON.parse(File.read(File.join(ARGV[0], "releases", "#{name}.json"))) }
  referenced = documents.map { |document| document.fetch("release") }.uniq.sort
  identities = JSON.parse(ENV.fetch("RELEASE_ARTIFACTS")).map { |record| record.fetch("release") }.uniq.sort
  abort "release artifact identities do not cover channel bundle" unless referenced == identities
  abort "default/current release mismatch" unless documents.drop(1).all? { |document| document.fetch("release") == ARGV[1] }
' /out/bundle "$default_release"
bundle_digest=$(scripts/channel-bundle-digest.rb /out/bundle)

headers=(
  --header "Accept: application/vnd.github+json"
  --header "Authorization: Bearer ${AHAIRU_TOKEN}"
  --header "X-GitHub-Api-Version: 2022-11-28"
)
state_ref=automation/araihu-assets-state
state_path=.automation/araihu-assets/accepted-channel-v1.json
ref_status=$(curl --silent --show-error --output /tmp/accepted-state-ref.json --write-out '%{http_code}' \
  "${headers[@]}" "${GITHUB_API_URL}/repos/${AHAIRU_OWNER}/${AHAIRU_REPOSITORY}/git/ref/heads/${state_ref}")
test "$ref_status" = 200 || { echo "Dedicated accepted-state ref is unavailable (HTTP $ref_status)" >&2; exit 1; }
state_status=$(curl --silent --show-error --output /tmp/accepted-state.json --write-out '%{http_code}' \
  "${headers[@]}" "${GITHUB_API_URL}/repos/${AHAIRU_OWNER}/${AHAIRU_REPOSITORY}/contents/${state_path}?ref=${state_ref}")
case "$state_status" in
  200)
    decision=$(scripts/accepted-channel-state.rb /tmp/accepted-state.json "$bundle_digest" \
      "$GITHUB_REPOSITORY" "$GITHUB_REPOSITORY/.github/workflows/campaigns.yml" "$GITHUB_SERVER_URL" "$state_path")
    changed=$(ruby -rjson -e 'puts JSON.parse(ARGV[0]).fetch("changed")' "$decision")
    ;;
  404) changed=true ;;
  *) echo "Accepted-state read failed (HTTP $state_status)" >&2; exit 1 ;;
esac

RELEASE_ARTIFACTS="$release_artifacts" ruby -rjson -e '
  File.write("/out/plan.json", JSON.generate({
    changed: ARGV[0] == "true",
    bundle_digest: ARGV[1],
    runtime_release: ARGV[2],
    resolution_date: ARGV[3],
    release_artifacts: JSON.parse(ENV.fetch("RELEASE_ARTIFACTS")),
    state: {ref: "automation/araihu-assets-state", path: ".automation/araihu-assets/accepted-channel-v1.json"}
  }))
' "$changed" "$bundle_digest" "$default_release" "$resolution_date"
case "$changed" in true|false) ;; *) echo "invalid campaign changed value" >&2; exit 1 ;; esac
[[ "$bundle_digest" =~ ^[0-9a-f]{64}$ ]] || { echo "invalid campaign bundle digest" >&2; exit 1; }
printf 'changed=%s\ndigest=%s\n' "$changed" "$bundle_digest" > /out/provider-output
chmod 0644 /out/provider-output
echo "campaign plan: digest=$bundle_digest changed=$changed"
