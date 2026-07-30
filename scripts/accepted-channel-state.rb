#!/usr/bin/env ruby
# Validates one GitHub Contents API state response and compares its current digest.
require "base64"
require "date"
require "json"

abort "usage: accepted-channel-state.rb <response> <bundle-digest> <source-repository> <source-workflow> <server-url> <state-path>" unless ARGV.length == 6
response_path, bundle_digest, source_repository, source_workflow, server_url, state_path = ARGV
sha256 = /\A[0-9a-f]{64}\z/
git_sha = /\A[0-9a-f]{40}\z/
release_tag = /\Av(?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*)(?:[-+][0-9A-Za-z.-]+)?\z/
abort "invalid candidate bundle digest" unless bundle_digest.match?(sha256)
abort "invalid source repository" unless source_repository.match?(%r{\A[^/\s]+/[^/\s]+\z})
abort "invalid source workflow" unless source_workflow == "#{source_repository}/.github/workflows/campaigns.yml"
abort "invalid server URL" unless server_url.match?(%r{\Ahttps://[^/]+\z})
abort "invalid state path" unless state_path == ".automation/araihu-assets/accepted-channel-v1.json"

wrapper = JSON.parse(File.binread(response_path))
abort "state response is not a file" unless wrapper.fetch("type") == "file"
abort "state response path mismatch" unless wrapper.fetch("path") == state_path
abort "state response encoding is not base64" unless wrapper.fetch("encoding") == "base64"
blob_sha = wrapper.fetch("sha")
abort "invalid state blob SHA" unless blob_sha.match?(git_sha)
state = JSON.parse(Base64.decode64(wrapper.fetch("content")))

expected_keys = %w[
  bundleDigest channelArtifactId channelArtifactSha256 channelArtifactUrl
  release releaseSha256 releaseUrl resolutionDate schemaVersion
  sourceRepository sourceRevision sourceWorkflow
]
abort "accepted state fields differ from schema v1" unless state.keys.sort == expected_keys.sort
abort "accepted state schema mismatch" unless state.fetch("schemaVersion") == 1
accepted_digest = state.fetch("bundleDigest")
abort "invalid accepted bundle digest" unless accepted_digest.match?(sha256)
artifact_id = state.fetch("channelArtifactId")
abort "invalid channel artifact ID" unless artifact_id.is_a?(Integer) && artifact_id.positive?
artifact_sha = state.fetch("channelArtifactSha256")
abort "invalid channel artifact SHA-256" unless artifact_sha.match?(sha256)
artifact_url = state.fetch("channelArtifactUrl")
artifact_pattern = %r{\A#{Regexp.escape(server_url)}/#{Regexp.escape(source_repository)}/actions/runs/[1-9][0-9]*/artifacts/#{artifact_id}\z}
abort "invalid channel artifact URL" unless artifact_url.match?(artifact_pattern)
abort "accepted state source repository mismatch" unless state.fetch("sourceRepository") == source_repository
abort "accepted state source workflow mismatch" unless state.fetch("sourceWorkflow") == source_workflow
abort "invalid accepted source revision" unless state.fetch("sourceRevision").match?(git_sha)
release = state.fetch("release")
abort "invalid accepted release" unless release.match?(release_tag)
release_sha = state.fetch("releaseSha256")
abort "invalid accepted release SHA-256" unless release_sha.match?(sha256)
release_url = state.fetch("releaseUrl")
abort "invalid accepted release URL" unless release_url.start_with?("#{server_url}/#{source_repository}/releases/download/#{release}/")
date = state.fetch("resolutionDate")
abort "invalid accepted resolution date" unless Date.iso8601(date).iso8601 == date

puts JSON.generate({"changed" => accepted_digest != bundle_digest, "sha" => blob_sha})
