#!/usr/bin/env bash
set -euo pipefail
repo=${1:-.}

ruby -ryaml - "$repo" <<'RUBY'
repo = File.expand_path(ARGV.fetch(0))
fail_contract = ->(message) { abort "release workflow contract: #{message}" }
require_contract = ->(condition, message) { fail_contract.call(message) unless condition }
load_workflow = ->(name) { YAML.safe_load(File.read(File.join(repo, ".github/workflows", name)), aliases: false) }
trigger = ->(workflow) { workflow["on"] || workflow[true] }
steps = ->(workflow) { workflow.fetch("jobs").values.flat_map { |job| job.fetch("steps", []) } }
run_text = ->(workflow) { steps.call(workflow).map { |step| step["run"] }.compact.join("\n") }
read = ->(path) { File.read(File.join(repo, path)) }

release = load_workflow.call("release.yml")
fanout = load_workflow.call("release-fanout.yml")
campaigns = load_workflow.call("campaigns.yml")
ci = load_workflow.call("ci.yml")

require_contract.call(trigger.call(release).keys == ["push"] && trigger.call(release).dig("push", "tags") == ["v*"], "release trigger differs")
require_contract.call(release["permissions"] == {"contents"=>"read"}, "release default permission differs")
release_jobs = release.fetch("jobs")
require_contract.call(release_jobs.keys == %w[publish fanout], "release jobs differ")
publish = release_jobs.fetch("publish")
require_contract.call(publish["permissions"] == {"contents"=>"write"}, "release permission differs")
require_contract.call(publish["outputs"] == {"release"=>"${{ steps.release.outputs.tag }}"}, "release output is not verified tag")
fanout_call = release_jobs.fetch("fanout")
require_contract.call(fanout_call["needs"] == "publish" && fanout_call["uses"] == "./.github/workflows/release-fanout.yml", "release fan-out handoff differs")
require_contract.call(fanout_call["with"] == {"release"=>"${{ needs.publish.outputs.release }}"}, "release fan-out input differs")

release_steps = steps.call(release)
release_runs = run_text.call(release)
require_contract.call(release_steps.any? { |step| step["uses"]&.start_with?("actions/checkout@") && step.dig("with", "ref") == "${{ github.sha }}" }, "release checkout is not immutable")
tag_step = release_steps.find { |step| step["id"] == "release" }
tag_run = tag_step&.fetch("run", "").to_s
require_contract.call(tag_run.include?('event_sha=$(git rev-parse "${GITHUB_SHA}^{commit}")') && tag_run.include?('tag_sha=$(git rev-parse "refs/tags/${GITHUB_REF_NAME}^{commit}")') && tag_run.include?('test "$tag_sha" = "$event_sha"'), "tag-to-event binding differs")
preflight = release_steps.find { |step| step["id"] == "fanout-credentials" }
require_contract.call(preflight && preflight["env"] == {"ARAIHU_ASSETS_APP_ID"=>"${{ secrets.ARAIHU_ASSETS_APP_ID }}", "ARAIHU_ASSETS_APP_PRIVATE_KEY"=>"${{ secrets.ARAIHU_ASSETS_APP_PRIVATE_KEY }}"}, "credential preflight inputs differ")
require_contract.call(preflight["run"].include?('[[ -z "$ARAIHU_ASSETS_APP_ID" ]]') && preflight["run"].include?('[[ -z "$ARAIHU_ASSETS_APP_PRIVATE_KEY" ]]'), "credential preflight does not reject either missing secret")
materialize = release_steps.index { |step| step["run"].to_s.include?("release-build .dagger-inputs") }
preflight_index = release_steps.index(preflight)
build_index = release_steps.index { |step| step["run"].to_s.include?("dagger call release-bundle") }
publish_index = release_steps.index { |step| step["run"].to_s.include?("dagger call release-publish") }
require_contract.call([materialize, preflight_index, build_index, publish_index].all? && materialize < preflight_index && preflight_index < build_index && build_index < publish_index, "release input/credential/effect order differs")
require_contract.call(release_steps.any? { |step| step["uses"]&.start_with?("actions/upload-artifact@") && step.dig("with", "name") == "release-bundle-${{ steps.release.outputs.tag }}" }, "immutable release artifact differs")
require_contract.call(release_steps.any? { |step| step["uses"]&.start_with?("actions/upload-artifact@") && step.dig("with", "name") == "latest-candidate-${{ steps.release.outputs.tag }}" }, "latest candidate artifact differs")

release_build = read.call("scripts/dagger/release-build.sh")
%w["env -u HTTPS_PROXY -u HTTP_PROXY make check" "env -u HTTPS_PROXY -u HTTP_PROXY make proof-check" "go test ./... -count=1" "go vet ./..." "make verify" "make release"].each do |contract|
  require_contract.call(release_build.include?(contract.delete_prefix('"').delete_suffix('"')), "release build gate missing: #{contract}")
end
require_contract.call(release_build.include?('gh release download "$default_release"') && release_build.include?('sha256sum --check --strict "$default_download/archive.sha256"'), "promoted release provenance differs")
require_contract.call(release_build.include?('ruby scripts/validate-release-archive-members.rb "$default_download/archive.members"'), "release archive collision validation missing")
require_contract.call(release_build.include?('campaigns publish --date 1970-01-01 --output "$channel_candidate"') && release_build.include?('"$channel_candidate/releases/latest.json"'), "latest channel generation differs")
require_contract.call(!release_build.include?("default.json") && !release_build.include?("current.json"), "release writes mutable default/current channels")

release_publish = read.call("scripts/dagger/release-publish.sh")
preflight_loop = release_publish.index('for asset in "${assets[@]}"; do')
missing_record = release_publish.index('missing+=("$asset")')
upload_loop = release_publish.index('for asset in "${missing[@]}"; do')
upload_call = release_publish.index('gh release upload "$RELEASE_TAG" "$asset"')
require_contract.call([preflight_loop, missing_record, upload_loop, upload_call].all? && preflight_loop < missing_record && missing_record < upload_loop && upload_loop < upload_call, "release upload is not preflight-first")
require_contract.call(release_publish.scan("gh release upload").length == 1, "release has multiple upload paths")
require_contract.call(!release_publish.include?("--clobber"), "release publication can clobber assets")
require_contract.call(release_publish.include?('sha256sum --check --strict SHA256SUMS'), "release assets are not hash checked")

fanout_on = trigger.call(fanout)
require_contract.call(fanout_on.keys.sort == %w[workflow_call workflow_dispatch], "fan-out triggers differ")
%w[workflow_call workflow_dispatch].each do |kind|
  input = fanout_on.dig(kind, "inputs", "release")
  require_contract.call(input == {"required"=>true, "type"=>"string"} || (kind == "workflow_dispatch" && input["description"] && input["required"] == true && input["type"] == "string"), "#{kind} release input differs")
end
require_contract.call(fanout["permissions"] == {"contents"=>"read"}, "fan-out permission differs")
require_contract.call(fanout["concurrency"] == {"group"=>"release-consumer-fanout-${{ inputs.release }}", "cancel-in-progress"=>false}, "fan-out retry serialization differs")
fanout_steps = steps.call(fanout)
verify_index = fanout_steps.index { |step| step["run"].to_s.include?("dagger call release-fanout-plan") }
consumers_index = fanout_steps.index { |step| step["id"] == "consumers" }
token_index = fanout_steps.index { |step| step["id"] == "app-token" }
dispatch_index = fanout_steps.index { |step| step["run"].to_s.include?("dagger call release-fanout ") }
require_contract.call([verify_index, consumers_index, token_index, dispatch_index].all? && verify_index < consumers_index && consumers_index < token_index && token_index < dispatch_index, "fan-out must verify before scoped token and dispatch")
token = fanout_steps.fetch(token_index)
require_contract.call(token["with"] == {"app-id"=>"${{ secrets.ARAIHU_ASSETS_APP_ID }}", "private-key"=>"${{ secrets.ARAIHU_ASSETS_APP_PRIVATE_KEY }}", "owner"=>"araihu", "repositories"=>"${{ steps.consumers.outputs.repositories }}", "permission-contents"=>"write"}, "fan-out token scope differs")

fanout_plan = read.call("scripts/dagger/release-fanout-plan.sh")
require_contract.call(fanout_plan.include?('gh release download "$RELEASE"') && fanout_plan.include?("SHA256SUMS") && fanout_plan.include?("sha256sum --check --strict /tmp/release-dispatch.sha256"), "fan-out published bytes are not verified")
require_contract.call(fanout_plan.include?('release_document.fetch("release") == release') && fanout_plan.include?('commits/${RELEASE}'), "fan-out tag/metadata provenance differs")
manifest = YAML.safe_load(read.call("manifests/release-consumers.yaml"), aliases: false)
require_contract.call(manifest == {"schema_version"=>1, "owner"=>"araihu", "repositories"=>%w[goshtoso goshtoso-app-shells goshtoso-charts manja paje xisnove]}, "fan-out enrollment differs")
consumer = read.call("scripts/release-consumer-fanout.rb")
require_contract.call(consumer.include?('"event_type" => "araihu-assets-released"'), "fan-out event differs")
require_contract.call(consumer.include?("results = repositories.map") && consumer.include?('failed = results.reject'), "fan-out must attempt every consumer and aggregate failures on every retry")
consumer_test = read.call("scripts/release-consumer-fanout_test.sh")
require_contract.call(consumer_test.scan('"$fanout" dispatch "$manifest" "$metadata"').length >= 3 && consumer_test.include?('test -f "$retry_log/$repository.json"'), "manual retry must redispatch every enrolled consumer")

campaign_on = trigger.call(campaigns)
require_contract.call(campaign_on.dig("schedule", 0, "cron") == "0 0 * * *", "campaign schedule differs")
require_contract.call(campaign_on.dig("push", "branches") == ["main"] && campaign_on.dig("push", "paths") == ["manifests/campaigns.yaml", "manifests/default.yaml"], "campaign push routing differs")
require_contract.call(campaigns["permissions"] == {"contents"=>"read"} && campaigns["concurrency"] == {"group"=>"assets-channel-deployment", "cancel-in-progress"=>false}, "campaign permission/concurrency differs")
campaign_steps = steps.call(campaigns)
upload = campaign_steps.find { |step| step["id"] == "changed" }
dispatch = campaign_steps.find { |step| step["run"].to_s.include?("dagger call campaign-dispatch") }
require_contract.call(upload["if"] == "steps.plan.outputs.changed == 'true'" && dispatch["if"] == "steps.plan.outputs.changed == 'true'", "unchanged campaign must not upload or dispatch")
require_contract.call(campaign_steps.none? { |step| step.dig("with", "name").to_s.start_with?("accepted-channel-") }, "Actions artifact cannot represent accepted state")

campaign_plan = read.call("scripts/dagger/campaign-plan.sh")
require_contract.call(campaign_plan.include?("contents/${state_path}?ref=${state_ref}") && campaign_plan.include?("automation/araihu-assets-state") && campaign_plan.include?(".automation/araihu-assets/accepted-channel-v1.json"), "durable accepted-state read differs")
require_contract.call(campaign_plan.include?('sha256sum --check --strict "$download/archive.sha256"') && campaign_plan.include?('sha256sum --check --strict "$latest_download/latest.sha256"'), "campaign published hashes are not verified")
require_contract.call(campaign_plan.include?('ruby scripts/validate-release-archive-members.rb "$download/archive.members"'), "campaign archive collision validation missing")
state = read.call("scripts/accepted-channel-state.rb")
require_contract.call(state.include?('accepted_digest != bundle_digest') && state.include?('state.fetch("sourceRepository") == source_repository') && state.include?('state.fetch("sourceWorkflow") == source_workflow'), "accepted-state digest/provenance differs")
campaign_dispatch = read.call("scripts/dagger/campaign-dispatch.sh")
%w[release_artifacts runtime_release candidate_bundle_digest channel_artifact_id channel_artifact_sha256].each { |field| require_contract.call(campaign_dispatch.include?(field), "campaign dispatch omits #{field}") }
require_contract.call(!campaign_dispatch.include?("--request PUT") && !campaign_dispatch.include?("contents/${STATE_PATH}"), "Assets must not accept its own candidate")

archive = read.call("scripts/validate-release-archive-members.rb")
require_contract.call(archive.include?("exact.key?(path)") && archive.include?("unicode_normalize(:nfc).downcase(:fold)") && archive.include?('components.include?("..")'), "archive collision/traversal contract differs")
dagger_source = read.call(".dagger/src/index.ts")
%w[campaignPlan campaignDispatch releaseBundle releasePublish releaseFanoutPlan releaseFanout].each { |name| require_contract.call(dagger_source.include?(name), "missing Dagger function #{name}") }
require_contract.call(dagger_source.scan('@func({ cache: "never" })').length >= 6 && dagger_source.include?("effect_nonce"), "fresh reads/effects cache policy differs")
require_contract.call(read.call("scripts/dagger/ci.sh").include?("./scripts/check-release-workflows_test.sh"), "CI omits release contract tests")

[release, fanout, campaigns].each do |workflow|
  steps.call(workflow).select { |step| step["run"].to_s.include?("dagger call") }.each do |step|
    require_contract.call(!step["run"].include?("${{"), "provider expression reached Dagger CLI")
  end
end
puts "release workflows: provenance, publication, campaign, and fan-out contracts valid"
RUBY
