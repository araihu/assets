#!/usr/bin/env bash
set -euo pipefail
test -n "$NETWORK_NONCE"

ruby -e '
  pattern = /\Av(?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*)(?:-(?:0|[1-9][0-9]*|[0-9]*[A-Za-z-][0-9A-Za-z-]*)(?:\.(?:0|[1-9][0-9]*|[0-9]*[A-Za-z-][0-9A-Za-z-]*))*)?(?:\+[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?\z/
  abort "release must be a strict SemVer tag" unless ARGV[0].match?(pattern)
' "$RELEASE"

published=/tmp/published-release
mkdir -p "$published" /out
GH_TOKEN="$RELEASE_GITHUB_TOKEN" gh release view "$RELEASE" --repo "$GITHUB_REPOSITORY" --json tagName,isDraft > /tmp/github-release.json
ruby -rjson -e '
  document = JSON.parse(File.read(ARGV[0]))
  abort "GitHub Release tag mismatch" unless document.fetch("tagName") == ARGV[1]
  abort "GitHub Release is still a draft" if document.fetch("isDraft")
' /tmp/github-release.json "$RELEASE"
archive_name="araihu-assets-${RELEASE}.tar.gz"
GH_TOKEN="$RELEASE_GITHUB_TOKEN" gh release download "$RELEASE" --repo "$GITHUB_REPOSITORY" \
  --pattern "$archive_name" --pattern release.json --pattern SHA256SUMS --dir "$published"
assets_revision=$(GH_TOKEN="$RELEASE_GITHUB_TOKEN" gh api "repos/${GITHUB_REPOSITORY}/commits/${RELEASE}" --jq .sha)
ruby -rjson - "$published" "$archive_name" "$RELEASE" "$assets_revision" /out/release-dispatch.json /tmp/release-dispatch.sha256 <<'RUBY'
root, archive_name, release, revision, metadata_path, checks_path = ARGV
abort "assets repository mismatch" unless ENV.fetch("GITHUB_REPOSITORY") == "araihu/assets"
abort "assets revision is not a commit SHA" unless revision.match?(/\A[0-9a-f]{40}\z/)
lines = File.readlines(File.join(root, "SHA256SUMS"), chomp: true)
entries = lines.to_h do |line|
  match = line.match(/\A([0-9a-f]{64})  ([^\x00-\x1f\x7f]+)\z/) or abort "invalid SHA256SUMS record"
  [match[2], match[1]]
end
abort "duplicate SHA256SUMS record" unless entries.length == lines.length
required = [archive_name, "release.json"]
abort "missing dispatch checksum" unless required.all? { |name| entries.key?(name) }
release_document = JSON.parse(File.read(File.join(root, "release.json")))
abort "release.json tag mismatch" unless release_document.fetch("release") == release
File.write(checks_path, required.map { |name| "#{entries.fetch(name)}  #{File.join(root, name)}\n" }.join)
File.write(metadata_path, JSON.generate({
  assets_repository: "araihu/assets", assets_revision: revision, release: release,
  release_url: "https://github.com/araihu/assets/releases/download/#{release}/#{archive_name}",
  release_sha256: entries.fetch(archive_name), release_json_sha256: entries.fetch("release.json")
}))
RUBY
sha256sum --check --strict /tmp/release-dispatch.sha256
scripts/release-consumer-fanout.rb repositories manifests/release-consumers.yaml > /out/repositories
echo "fan-out plan: release ${RELEASE} verified"
