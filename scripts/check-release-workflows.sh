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
require_contract(release_steps.any? { |step| step["uses"]&.start_with?("actions/checkout@") && step.dig("with", "ref") == "${{ github.ref }}" }, "release checkout must use exact github.ref")
require_contract(release_steps.any? { |step| step["uses"]&.start_with?("actions/setup-go@") && step.dig("with", "go-version") == "1.26.5" }, "release must install Go 1.26.5")
require_contract(release_runs.include?("env -u HTTPS_PROXY -u HTTP_PROXY make check"), "release must run offline check gate")
require_contract(release_runs.include?("env -u HTTPS_PROXY -u HTTP_PROXY make proof-check"), "release must run offline proof gate")
require_contract(release_runs.include?("go test ./... -count=1") && release_runs.include?("go vet ./...") && release_runs.include?("make verify") && release_runs.include?("make release"), "release must run complete verification gate")
require_contract(release_runs.include?("sha256sum --check --strict checksums.txt"), "release must verify generated checksums")
require_contract(release_runs.include?("gh release create") && release_runs.include?("gh release upload"), "release must create or safely update matching GitHub Release")
require_contract(release_steps.any? { |step| step["uses"]&.start_with?("actions/upload-artifact@") && step.dig("with", "name") == "release-bundle-${{ steps.release.outputs.tag }}" }, "release must upload immutable release bundle")
require_contract(release_steps.any? { |step| step["uses"]&.start_with?("actions/upload-artifact@") && step.dig("with", "name") == "latest-candidate-${{ steps.release.outputs.tag }}" }, "release must upload latest candidate separately")
require_contract(release_runs.include?("latest.json"), "release must write latest candidate")
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
require_contract(campaign_job["permissions"] == {"actions" => "read", "contents" => "read"}, "campaign job permissions must be actions read and contents read only")
require_contract(campaign_runs.include?("releases/${default_release}/release.json") && campaign_runs.include?("gh release download"), "campaign workflow must materialize older promoted release snapshots")
require_contract(campaign_runs.include?("campaigns publish --date") && campaign_runs.include?("--output"), "campaign workflow must build channel bundle with CLI")
require_contract(campaign_runs.include?("accepted-channel-${channel_digest}"), "campaign workflow must compare accepted channel digest")
require_contract(campaign_steps.any? { |step| step["id"] == "changed" && step["if"] == "steps.channel.outputs.changed == 'true'" }, "campaign change gate is missing")
app_step = campaign_steps.find { |step| step["uses"]&.start_with?("actions/create-github-app-token@") }
require_contract(app_step, "campaign workflow must mint GitHub App token")
require_contract(app_step["if"] == "steps.channel.outputs.changed == 'true'", "GitHub App token must only be minted on change")
require_contract(app_step.fetch("with", {}).keys.sort == ["app-id", "owner", "permission-contents", "private-key", "repositories"], "GitHub App token inputs grant extra scope")
require_contract(app_step.dig("with", "owner") == "${{ vars.AHAIRU_OWNER }}", "GitHub App token must target selected Ahairu owner")
require_contract(app_step.dig("with", "repositories") == "${{ vars.AHAIRU_REPOSITORY }}", "GitHub App token must target selected Ahairu repository")
require_contract(app_step.dig("with", "permission-contents") == "write", "GitHub App token must grant only dispatch-compatible contents permission")
require_contract(campaign_runs.include?("/dispatches") && campaign_runs.include?("channel_artifact_id") && campaign_runs.include?("channel_artifact_sha256") && campaign_runs.include?("release_sha256"), "Ahairu dispatch must carry immutable artifact identity and hashes")
["release.yml", "campaigns.yml"].each do |name|
  source = File.read(File.join(repo, ".github", "workflows", name))
  require_contract(!source.match?(/CLOUDFLARE/i), "#{name} must not reference Cloudflare credentials")
end
assert_pinned_actions(campaigns, "campaign")

require_contract(run_text(ci).include?("./scripts/check-release-workflows_test.sh"), "CI must run release workflow structure tests")
assert_pinned_actions(ci, "CI")

puts "release workflows: guarded release and campaign contracts are valid"
RUBY
