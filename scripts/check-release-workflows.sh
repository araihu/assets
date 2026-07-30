#!/usr/bin/env bash
# Validates release/campaign workflow structure without network access or yq.
set -euo pipefail

repo=${1:-.}

ruby - "$repo" <<'RUBY'
require "yaml"

repo = File.expand_path(ARGV.fetch(0))

def fail_contract(message)
  warn "release workflow contract: #{message}"
  exit 1
end

def require_contract(condition, message)
  fail_contract(message) unless condition
end

def load_workflow(repo, name)
  path = File.join(repo, ".github", "workflows", name)
  require_contract(File.file?(path), "missing #{path}")
  document = YAML.safe_load(File.read(path), aliases: false)
  require_contract(document.is_a?(Hash), "#{name} is not a YAML mapping")
  document
rescue Psych::Exception => error
  fail_contract("#{name} is invalid YAML: #{error.message}")
end

def triggers(workflow)
  # Psych follows YAML 1.1 and may decode an unquoted `on` key as true.
  workflow["on"] || workflow[true]
end

def job_steps(workflow)
  jobs = workflow["jobs"]
  require_contract(jobs.is_a?(Hash) && !jobs.empty?, "workflow jobs are missing")
  jobs.values.flat_map { |job| job.fetch("steps", []) }
end

def run_text(workflow)
  job_steps(workflow).map { |step| step["run"] }.compact.join("\n")
end

def action_uses(workflow)
  job_steps(workflow).map { |step| step["uses"] }.compact
end

def assert_pinned_actions(workflow, name)
  action_uses(workflow).each do |use|
    require_contract(use.match?(%r{\A[^/\s]+/[^@\s]+@[0-9a-f]{40}\z}), "#{name} action is not pinned by full commit SHA: #{use}")
  end
end

release = load_workflow(repo, "release.yml")
campaigns = load_workflow(repo, "campaigns.yml")
ci = load_workflow(repo, "ci.yml")

release_on = triggers(release)
require_contract(release_on.is_a?(Hash) && release_on.keys == ["push"], "release trigger must contain only push")
require_contract(release_on.dig("push", "tags") == ["v*"], "release tags must match v* only")
require_contract(release["permissions"] == {"contents" => "read"}, "release default permission must be contents read only")
release_jobs = release.fetch("jobs", {})
require_contract(release_jobs.length == 1, "release workflow must have one job")
release_job = release_jobs.values.first
require_contract(release_job["permissions"] == {"contents" => "write"}, "release publishing job needs contents write only")
release_steps = job_steps(release)
release_runs = run_text(release)
require_contract(release_steps.any? { |step| step["uses"]&.start_with?("actions/checkout@") && step.dig("with", "ref") == "${{ github.sha }}" }, "release checkout must use immutable github.sha")
require_contract(release_runs.include?('event_sha=$(git rev-parse "${GITHUB_SHA}^{commit}")') && release_runs.include?('git rev-parse "refs/tags/${GITHUB_REF_NAME}^{commit}"') && release_runs.include?('test "$tag_sha" = "$event_sha"'), "release must resolve the triggering tag to github.sha")
require_contract(release_steps.any? { |step| step["uses"]&.start_with?("actions/setup-go@") && step.dig("with", "go-version") == "1.26.5" }, "release must install Go 1.26.5")
require_contract(release_runs.include?("env -u HTTPS_PROXY -u HTTP_PROXY make check"), "release must run offline check gate")
require_contract(release_runs.include?("env -u HTTPS_PROXY -u HTTP_PROXY make proof-check"), "release must run offline proof gate")
require_contract(release_runs.include?("go test ./... -count=1") && release_runs.include?("go vet ./...") && release_runs.include?("make verify") && release_runs.include?("make release"), "release must run complete verification gate")
require_contract(release_runs.include?("sha256sum --check --strict checksums.txt"), "release must verify generated checksums")
require_contract(release_runs.include?("gh release create") && release_runs.include?("gh release upload"), "release must create or safely update matching GitHub Release")
release_publish = release_steps.map { |step| step["run"] }.compact.find { |run| run.include?("gh release create") }
require_contract(release_publish, "release publication step is missing")
preflight_loop = release_publish.index('for asset in "${assets[@]}"; do')
missing_record = release_publish.index('missing+=("$asset")')
upload_loop = release_publish.index('for asset in "${missing[@]}"; do')
upload_call = release_publish.index('gh release upload "$TAG" "$asset"')
require_contract(preflight_loop && missing_record && upload_loop && upload_call && preflight_loop < missing_record && missing_record < upload_loop && upload_loop < upload_call, "release must preflight every existing asset before uploading missing assets")
require_contract(release_publish.scan("gh release upload").length == 1, "release upload must exist only in post-preflight loop")
require_contract(!release_publish.include?("--clobber"), "release publication must never clobber assets")
require_contract(release_steps.any? { |step| step["uses"]&.start_with?("actions/upload-artifact@") && step.dig("with", "name") == "release-bundle-${{ steps.release.outputs.tag }}" }, "release must upload immutable release bundle")
require_contract(release_steps.any? { |step| step["uses"]&.start_with?("actions/upload-artifact@") && step.dig("with", "name") == "latest-candidate-${{ steps.release.outputs.tag }}" }, "release must upload latest candidate separately")
require_contract(release_runs.include?('campaigns publish --date "$CHANNEL_DATE" --output "$channel_candidate"'), "release must generate latest through the CLI publication path")
require_contract(release_runs.include?('"$channel_candidate/releases/latest.json"'), "release must stage the resolved latest channel")
require_contract(!release_runs.include?('dist/release.json "$latest/latest.json"'), "release must not rename release.json as latest.json")
release_assets = release_publish[/assets=\((.*?)\n\s*\)/m, 1].to_s
require_contract(release_assets.include?('"$BUNDLE/latest.json"'), "GitHub Release must publish latest.json")
require_contract(release_runs.include?('latest.json checksums.txt > SHA256SUMS'), "release SHA256SUMS must cover latest.json")
require_contract(!release_runs.include?("default.json") && !release_runs.include?("current.json"), "release must not write default or current channels")
assert_pinned_actions(release, "release")

campaign_on = triggers(campaigns)
require_contract(campaign_on.dig("schedule", 0, "cron") == "0 0 * * *", "campaign schedule must be 00:00 UTC")
dispatch_date = campaign_on.dig("workflow_dispatch", "inputs", "date")
require_contract(dispatch_date.is_a?(Hash) && dispatch_date["required"] == false && dispatch_date["type"] == "string", "campaign manual date input is incomplete")
require_contract(campaign_on.dig("push", "branches") == ["main"], "campaign push branch must be main")
require_contract(campaign_on.dig("push", "paths") == ["manifests/campaigns.yaml", "manifests/default.yaml"], "campaign push paths must be exact")
require_contract(campaigns["permissions"] == {"contents" => "read"}, "campaign default permission must be contents read only")
concurrency = campaigns["concurrency"]
require_contract(concurrency.is_a?(Hash) && concurrency["group"] == "assets-channel-deployment" && concurrency["cancel-in-progress"] == false, "campaign concurrency must be one non-cancelling deployment group")
require_contract(campaigns.fetch("jobs", {}).values.none? { |job| job.key?("concurrency") }, "campaign jobs must not define additional concurrency groups")
campaign_steps = job_steps(campaigns)
campaign_runs = run_text(campaigns)
campaign_job = campaigns.fetch("jobs", {}).values.first
require_contract(campaign_job["permissions"] == {"contents" => "read"}, "campaign job permission must be contents read only")
require_contract(campaign_runs.include?('latest_release=$(gh release view') && campaign_runs.include?('materialize_release "$default_release"'), "campaign workflow must require the promoted GitHub Release before resolution")
require_contract(campaign_runs.include?('gh release download "$latest_release"') && campaign_runs.include?("--pattern latest.json"), "campaign workflow must download published latest channel inputs")
require_contract(campaign_runs.include?('stage_snapshot "$default_release" "releases/${default_release}"') && campaign_runs.include?("releases/latest/latest.json"), "campaign workflow must materialize promoted and latest tagged snapshots")
require_contract(campaign_runs.include?("campaign/v1.js") && campaign_runs.include?('sha256sum --check --strict "$download/archive.sha256"') && campaign_runs.include?('sha256sum --check --strict "$latest_download/latest.sha256"') && campaign_runs.include?("unsafe release archive member"), "campaign workflow must hash-check and safely extract tagged runtime inputs")
require_contract(!campaign_runs.include?("dist/"), "campaign workflow must never source channel bytes from untagged dist")
require_contract(campaign_runs.include?("campaigns publish --date") && campaign_runs.include?("--output"), "campaign workflow must build channel bundle with CLI")
require_contract(campaign_runs.include?('scripts/channel-bundle-digest.rb "$bundle"'), "campaign workflow must compute canonical four-file bundle digest")
digest_script = File.join(repo, "scripts", "channel-bundle-digest.rb")
require_contract(File.file?(digest_script) && File.executable?(digest_script), "canonical bundle digest helper is missing or not executable")
state_script = File.join(repo, "scripts", "accepted-channel-state.rb")
require_contract(File.file?(state_script) && File.executable?(state_script), "accepted-state validator is missing or not executable")
app_step = campaign_steps.find { |step| step["uses"]&.start_with?("actions/create-github-app-token@") }
require_contract(app_step, "campaign workflow must mint GitHub App token")
require_contract(!app_step.key?("if"), "GitHub App token must be minted before durable-state comparison")
require_contract(app_step.fetch("with", {}).keys.sort == ["app-id", "owner", "permission-contents", "private-key", "repositories"], "GitHub App token inputs grant extra scope")
require_contract(app_step.dig("with", "owner") == "${{ vars.AHAIRU_OWNER }}", "GitHub App token must target selected Ahairu owner")
require_contract(app_step.dig("with", "repositories") == "${{ vars.AHAIRU_REPOSITORY }}", "GitHub App token must target selected Ahairu repository")
require_contract(app_step.dig("with", "permission-contents") == "write", "GitHub App token must grant only dispatch-compatible contents permission")
state_step = campaign_steps.find { |step| step["id"] == "state" }
require_contract(state_step, "durable accepted-state comparison step is missing")
require_contract(state_step.dig("env", "GH_TOKEN") == "${{ steps.app-token.outputs.token }}", "durable state must use selected-repository App token")
require_contract(state_step.dig("env", "STATE_REF") == "automation/araihu-assets-state", "durable state ref must be dedicated and fixed")
require_contract(state_step.dig("env", "STATE_PATH") == ".automation/araihu-assets/accepted-channel-v1.json", "durable state path must be dedicated and fixed")
state_run = state_step["run"].to_s
require_contract(state_run.include?("contents/${STATE_PATH}?ref=${STATE_REF}"), "durable state must read one fixed Contents API document")
require_contract(state_run.include?('scripts/accepted-channel-state.rb "$state_response" "$BUNDLE_DIGEST"'), "durable state validator and current bundle comparison are missing")
state_script_source = File.read(state_script)
require_contract(state_script_source.include?('state.fetch("schemaVersion") == 1') && state_script_source.include?('accepted_digest != bundle_digest'), "durable state schema and current bundle comparison are missing")
require_contract(state_script_source.include?('state.fetch("channelArtifactId")') && state_script_source.include?('state.fetch("channelArtifactSha256")'), "durable state artifact identity validation is missing")
require_contract(state_script_source.include?('state.fetch("sourceRepository") == source_repository') && state_script_source.include?('state.fetch("sourceWorkflow") == source_workflow'), "durable state provenance validation is missing")
accepted_artifacts = campaign_steps.select { |step| step["uses"]&.start_with?("actions/upload-artifact@") && step.dig("with", "name").to_s.start_with?("accepted-channel-") }
require_contract(!campaign_runs.include?("/actions/artifacts") && accepted_artifacts.empty?, "historical Actions artifacts must not represent accepted state")
changed_upload = campaign_steps.find { |step| step["id"] == "changed" }
require_contract(changed_upload && changed_upload["if"] == "steps.state.outputs.changed == 'true'", "delivery artifact must use durable-state change gate")
dispatch_step = campaign_steps.find { |step| step["run"]&.include?("/dispatches") }
require_contract(dispatch_step && dispatch_step["if"] == "steps.state.outputs.changed == 'true'", "Ahairu dispatch must use durable-state change gate")
dispatch_run = dispatch_step["run"]
require_contract(dispatch_run.include?('"/repos/${AHAIRU_OWNER}/${AHAIRU_REPOSITORY}/dispatches"'), "Ahairu repository dispatch call is missing")
require_contract(!dispatch_run.include?("contents/${STATE_PATH}") && !dispatch_run.include?("accepted-state-update") && !dispatch_run.include?("--request PUT"), "Assets must never write durable accepted state")
require_contract(dispatch_run.include?("release_artifacts: releases,") && dispatch_run.include?("runtime_release: runtime_release,"), "Ahairu dispatch must carry every distinct release identity and runtime release")
require_contract(dispatch_run.include?("candidate_bundle_digest: bundle_digest,") && dispatch_run.include?('state_ref: ENV.fetch("STATE_REF"),') && dispatch_run.include?('state_path: ENV.fetch("STATE_PATH")'), "Ahairu dispatch must carry candidate digest and exact durable-state locator")
require_contract(campaign_runs.include?("channel_artifact_id") && campaign_runs.include?("channel_artifact_sha256") && campaign_runs.include?("release_sha256"), "Ahairu dispatch must carry immutable artifact identity and hashes")
["release.yml", "campaigns.yml"].each do |name|
  source = File.read(File.join(repo, ".github", "workflows", name))
  require_contract(!source.match?(/CLOUDFLARE/i), "#{name} must not reference Cloudflare credentials")
end
assert_pinned_actions(campaigns, "campaign")

require_contract(run_text(ci).include?("./scripts/check-release-workflows_test.sh"), "CI must run release workflow structure tests")
assert_pinned_actions(ci, "CI")

puts "release workflows: guarded release and campaign contracts are valid"
RUBY
