#!/usr/bin/env ruby
# frozen_string_literal: true

require "json"
require "open3"
require "yaml"

SEMVER_TAG = /\Av(?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*)(?:-(?:0|[1-9][0-9]*|[0-9]*[A-Za-z-][0-9A-Za-z-]*)(?:\.(?:0|[1-9][0-9]*|[0-9]*[A-Za-z-][0-9A-Za-z-]*))*)?(?:\+[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?\z/
SHA1 = /\A[0-9a-f]{40}\z/
SHA256 = /\A[0-9a-f]{64}\z/
REPOSITORY = /\A[a-z0-9](?:[a-z0-9._-]*[a-z0-9])?\z/
PAYLOAD_KEYS = %w[
  assets_repository assets_revision release release_url release_sha256
  release_json_sha256
].freeze

def abort_usage
  abort "usage: release-consumer-fanout.rb repositories MANIFEST | dispatch MANIFEST METADATA_JSON"
end

def load_manifest(path)
  document = YAML.safe_load(File.read(path), permitted_classes: [], permitted_symbols: [], aliases: false)
  abort "release consumer manifest must be a mapping" unless document.is_a?(Hash)
  abort "release consumer manifest fields differ" unless document.keys.sort == %w[owner repositories schema_version]
  abort "unsupported release consumer manifest schema" unless document.fetch("schema_version") == 1

  owner = document.fetch("owner")
  repositories = document.fetch("repositories")
  abort "release consumer owner must be araihu" unless owner == "araihu"
  abort "release consumers must be a non-empty array" unless repositories.is_a?(Array) && !repositories.empty?
  abort "invalid release consumer repository" unless repositories.all? { |repository| repository.is_a?(String) && repository.match?(REPOSITORY) }
  abort "duplicate release consumer repository" unless repositories.uniq.length == repositories.length
  abort "release consumers must be sorted" unless repositories == repositories.sort

  [owner, repositories]
rescue Errno::ENOENT, Psych::Exception => error
  abort "invalid release consumer manifest: #{error.message}"
end

def load_metadata(path)
  document = JSON.parse(File.read(path))
  abort "release dispatch metadata must be a mapping" unless document.is_a?(Hash)
  abort "release dispatch metadata fields differ" unless document.keys.sort == PAYLOAD_KEYS.sort
  abort "assets repository must be araihu/assets" unless document.fetch("assets_repository") == "araihu/assets"
  abort "invalid assets revision" unless document.fetch("assets_revision").match?(SHA1)

  release = document.fetch("release")
  abort "release must be a strict SemVer tag" unless release.match?(SEMVER_TAG)
  expected_url = "https://github.com/araihu/assets/releases/download/#{release}/araihu-assets-#{release}.tar.gz"
  abort "release URL is not the immutable araihu/assets archive" unless document.fetch("release_url") == expected_url
  abort "invalid release archive SHA-256" unless document.fetch("release_sha256").match?(SHA256)
  abort "invalid release.json SHA-256" unless document.fetch("release_json_sha256").match?(SHA256)

  PAYLOAD_KEYS.to_h { |key| [key, document.fetch(key)] }
rescue Errno::ENOENT, JSON::ParserError => error
  abort "invalid release dispatch metadata: #{error.message}"
end

command, manifest_path, metadata_path, extra = ARGV
abort_usage if command.nil? || manifest_path.nil? || extra
owner, repositories = load_manifest(manifest_path)

case command
when "repositories"
  abort_usage if metadata_path
  puts repositories.join(",")
when "dispatch"
  abort_usage unless metadata_path
  client_payload = load_metadata(metadata_path)
  payload = JSON.generate({"event_type" => "araihu-assets-released", "client_payload" => client_payload})
  results = repositories.map do |repository|
    _stdout, _stderr, status = Open3.capture3(
      "gh", "api", "--method", "POST",
      "-H", "Accept: application/vnd.github+json",
      "-H", "X-GitHub-Api-Version: 2022-11-28",
      "/repos/#{owner}/#{repository}/dispatches", "--input", "-",
      stdin_data: payload
    )
    [repository, status.success?]
  end

  summary = ["### Release fallback consumer dispatch", ""]
  results.each do |repository, success|
    summary << "- `#{repository}`: #{success ? 'dispatched' : 'failed'}"
  end
  summary << ""
  summary << "Manual retry: run `Release consumer fan-out` with release `#{client_payload.fetch('release')}`."
  rendered = summary.join("\n") + "\n"
  print rendered
  File.open(ENV.fetch("GITHUB_STEP_SUMMARY"), "a") { |file| file.write(rendered) } if ENV["GITHUB_STEP_SUMMARY"]

  failed = results.reject { |_repository, success| success }.map(&:first)
  abort "failed consumers: #{failed.join(',')}" unless failed.empty?
else
  abort_usage
end
