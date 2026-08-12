#!/usr/bin/env ruby
# frozen_string_literal: true

require "json"

mode, output = ARGV
abort "usage: materialize-dagger-input.rb MODE OUTPUT" unless mode && output

fetch = ->(name) { ENV.fetch(name) }
nonce = -> { "#{fetch.call("GITHUB_RUN_ID")}-#{fetch.call("GITHUB_RUN_ATTEMPT")}" }

payload = case mode
          when "ci"
            cache_namespace = case fetch.call("GITHUB_EVENT_NAME")
                              when "pull_request" then "pr"
                              when "push", "workflow_dispatch" then "trusted"
                              else abort "unsupported CI event"
                              end
            {"cache_namespace" => cache_namespace, "run_nonce" => nonce.call}
          when "acquisition"
            abort "acquisition requires workflow_dispatch" unless fetch.call("GITHUB_EVENT_NAME") == "workflow_dispatch"
            {"refresh_nonce" => nonce.call}
          when "campaign-plan"
            abort "unsupported campaign event" unless %w[schedule workflow_dispatch push].include?(fetch.call("GITHUB_EVENT_NAME"))
            {
              "repository" => fetch.call("GITHUB_REPOSITORY"),
              "ahairu_owner" => fetch.call("AHAIRU_OWNER"),
              "ahairu_repository" => fetch.call("AHAIRU_REPOSITORY"),
              "date" => ENV.fetch("REQUESTED_DATE", ""),
              "github_server_url" => fetch.call("GITHUB_SERVER_URL"),
              "github_api_url" => fetch.call("GITHUB_API_URL"),
              "refresh_nonce" => nonce.call
            }
          when "campaign-dispatch"
            {
              "repository" => fetch.call("GITHUB_REPOSITORY"),
              "ahairu_owner" => fetch.call("AHAIRU_OWNER"),
              "ahairu_repository" => fetch.call("AHAIRU_REPOSITORY"),
              "github_server_url" => fetch.call("GITHUB_SERVER_URL"),
              "github_api_url" => fetch.call("GITHUB_API_URL"),
              "artifact_id" => fetch.call("CHANNEL_ARTIFACT_ID"),
              "artifact_url" => fetch.call("CHANNEL_ARTIFACT_URL"),
              "artifact_sha256" => fetch.call("CHANNEL_ARTIFACT_SHA256"),
              "source_revision" => fetch.call("GITHUB_SHA"),
              "effect_nonce" => nonce.call
            }
          when "release-build"
            abort "release build requires a tag push" unless fetch.call("GITHUB_EVENT_NAME") == "push" && fetch.call("GITHUB_REF_TYPE") == "tag"
            {
              "tag" => fetch.call("GITHUB_REF_NAME"),
              "source_revision" => fetch.call("GITHUB_SHA"),
              "repository" => fetch.call("GITHUB_REPOSITORY"),
              "refresh_nonce" => nonce.call
            }
          when "release-publish"
            {
              "tag" => fetch.call("GITHUB_REF_NAME"),
              "repository" => fetch.call("GITHUB_REPOSITORY"),
              "effect_nonce" => nonce.call
            }
          when "fanout-plan"
            {
              "release" => fetch.call("RELEASE"),
              "repository" => fetch.call("GITHUB_REPOSITORY"),
              "refresh_nonce" => nonce.call
            }
          when "fanout-dispatch"
            {"effect_nonce" => nonce.call}
          else
            abort "unsupported input mode"
          end

directory = File.dirname(output)
Dir.mkdir(directory, 0o700) unless Dir.exist?(directory)
File.open(output, File::WRONLY | File::CREAT | File::TRUNC, 0o600) do |file|
  file.write(JSON.generate(payload))
  file.write("\n")
end
