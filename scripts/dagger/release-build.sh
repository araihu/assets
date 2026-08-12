#!/usr/bin/env bash
set -euo pipefail
test -n "$NETWORK_NONCE"

ruby -e '
  abort "release must be a strict SemVer tag" unless ARGV[0].match?(/\Av(?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*)(?:-(?:0|[1-9][0-9]*|[0-9]*[A-Za-z-][0-9A-Za-z-]*)(?:\.(?:0|[1-9][0-9]*|[0-9]*[A-Za-z-][0-9A-Za-z-]*))*)?(?:\+[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?\z/)
  abort "source revision must be a full SHA-1" unless ARGV[1].match?(/\A[0-9a-f]{40}\z/)
' "$RELEASE_TAG" "$SOURCE_REVISION"
test -n "$ARAIHU_ASSETS_APP_ID" || { echo "ARAIHU_ASSETS_APP_ID is required" >&2; exit 1; }
test -n "$ARAIHU_ASSETS_APP_PRIVATE_KEY" || { echo "ARAIHU_ASSETS_APP_PRIVATE_KEY is required" >&2; exit 1; }
test "$(go version | awk '{print $3}')" = go1.26.5
go tool muamba verify --strict
go tool muamba generate-go --strict --check --dir internal/acquisition --output muamba_gen.go
env -u HTTPS_PROXY -u HTTP_PROXY make check
env -u HTTPS_PROXY -u HTTP_PROXY make proof-check
go test ./... -count=1
go vet ./...
make verify
make release
diff -ru --exclude=.cache /baseline /src
(cd dist && sha256sum --check --strict checksums.txt)

built_tag=$(ruby -rjson -e 'puts JSON.parse(File.read("dist/release.json")).fetch("release")')
test "$built_tag" = "$RELEASE_TAG"
archive_base="araihu-assets-${RELEASE_TAG}"
tar_archive="dist/releases/${archive_base}.tar.gz"
zip_archive="dist/releases/${archive_base}.zip"
test -f "$tar_archive"
test -f "$zip_archive"
mkdir -p /tmp/verify-tar /tmp/verify-zip
tar --extract --gzip --file "$tar_archive" --directory /tmp/verify-tar
unzip -q "$zip_archive" -d /tmp/verify-zip
(cd /tmp/verify-tar && sha256sum --check --strict checksums.txt)
(cd /tmp/verify-zip && sha256sum --check --strict checksums.txt)

mkdir -p /out/release-bundle /out/latest-candidate
default_release=$(ruby -ryaml -e 'puts YAML.safe_load(File.read("manifests/default.yaml")).fetch("release")')
if [[ "$default_release" != "$RELEASE_TAG" ]]; then
  default_download=/tmp/default-release/download
  default_root=/tmp/default-release/root
  default_archive="araihu-assets-${default_release}.tar.gz"
  mkdir -p "$default_download" "$default_root"
  gh release download "$default_release" --repo "$GITHUB_REPOSITORY" \
    --pattern "$default_archive" --pattern SHA256SUMS --dir "$default_download"
  default_sha256=$(ruby -e '
    name, path = ARGV
    matches = File.readlines(path, chomp: true).filter_map { |line| hash, candidate = line.split(/\s+/, 2); hash if candidate == name }
    abort "default release archive hash must appear exactly once" unless matches.length == 1 && matches.first.match?(/\A[0-9a-f]{64}\z/)
    puts matches.first
  ' "$default_archive" "$default_download/SHA256SUMS")
  printf '%s  %s\n' "$default_sha256" "$default_download/$default_archive" > "$default_download/archive.sha256"
  sha256sum --check --strict "$default_download/archive.sha256"
  tar --list --gzip --file "$default_download/$default_archive" > "$default_download/archive.members"
  ruby scripts/validate-release-archive-members.rb "$default_download/archive.members"
  tar --list --verbose --gzip --file "$default_download/$default_archive" | awk 'substr($1, 1, 1) != "-" { exit 1 }'
  tar --extract --gzip --file "$default_download/$default_archive" --directory "$default_root"
  (cd "$default_root" && sha256sum --check --strict checksums.txt)
  ruby -rjson -e 'document = JSON.parse(File.read(ARGV[0])); abort "default release metadata mismatch" unless document.fetch("release") == ARGV[1]' \
    "$default_root/release.json" "$default_release"
  mkdir -p "releases/${default_release}/campaign"
  for contract in release.json catalog.json themes.json campaigns.json; do
    install -m 0644 "$default_root/$contract" "releases/${default_release}/$contract"
  done
  install -m 0644 "$default_root/campaign/v1.js" "releases/${default_release}/campaign/v1.js"
fi

channel_candidate=/tmp/release-channel-candidate
go run ./cmd/araihu-assets campaigns publish --date 1970-01-01 --output "$channel_candidate"
ruby -rjson -e '
  document = JSON.parse(File.read(ARGV[0]))
  abort "latest release mismatch" unless document.fetch("release") == ARGV[1]
  abort "latest must resolve a baseline" unless document.fetch("source") == "default" && !document.key?("campaign")
' "$channel_candidate/releases/latest.json" "$RELEASE_TAG"
install -m 0644 "$tar_archive" "/out/release-bundle/${archive_base}.tar.gz"
install -m 0644 "$zip_archive" "/out/release-bundle/${archive_base}.zip"
install -m 0644 dist/release.json /out/release-bundle/release.json
install -m 0644 dist/checksums.txt /out/release-bundle/checksums.txt
install -m 0644 "$channel_candidate/releases/latest.json" /out/release-bundle/latest.json
(cd /out/release-bundle && sha256sum "${archive_base}.tar.gz" "${archive_base}.zip" release.json latest.json checksums.txt > SHA256SUMS)
(cd /out/release-bundle && sha256sum --check --strict SHA256SUMS)
install -m 0644 "$channel_candidate/releases/latest.json" /out/latest-candidate/latest.json
echo "release bundle: ${RELEASE_TAG} (${SOURCE_REVISION})"
