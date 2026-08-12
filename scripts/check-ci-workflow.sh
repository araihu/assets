#!/usr/bin/env bash
set -euo pipefail
repo=${1:-.}

test "$(ruby -rjson -e 'puts JSON.parse(File.read(ARGV[0])).fetch("engineVersion")' "$repo/dagger.json")" = v0.21.8
grep -Fx 'go 1.26.5' "$repo/go.mod"
if [[ -n "$(git -C "$repo" ls-files -- '.dagger/sdk' '.dagger/sdk/**')" ]]; then
  echo 'generated Dagger SDK fixtures must not be versioned' >&2
  exit 1
fi

ruby -rjson -ryaml - "$repo" <<'RUBY'
repo = File.expand_path(ARGV.fetch(0))
fail_contract = ->(message) { abort "CI workflow contract: #{message}" }
require_contract = ->(condition, message) { fail_contract.call(message) unless condition }
load_yaml = ->(path) { YAML.safe_load(File.read(File.join(repo, path)), aliases: false) }

dagger = JSON.parse(File.read(File.join(repo, "dagger.json")))
package = JSON.parse(File.read(File.join(repo, ".dagger/package.json")))
lock = JSON.parse(File.read(File.join(repo, ".dagger/package-lock.json")))
actionlint = YAML.safe_load(File.read(File.join(repo, ".github/actionlint.yaml")), aliases: false)
require_contract.call(dagger == {"name"=>"assets", "engineVersion"=>"v0.21.8", "sdk"=>{"source"=>"typescript"}, "source"=>".dagger"}, "Dagger config differs")
require_contract.call(actionlint == {"self-hosted-runner"=>{"labels"=>["hostinger-vps-pr", "hostinger-vps-trusted"]}}, "actionlint runner label catalog differs")
require_contract.call(package.dig("dependencies", "@dagger.io/dagger") == "./sdk", "TypeScript SDK must resolve from ./sdk")
require_contract.call(package.dig("dependencies", "typescript") == "6.0.3", "TypeScript runtime version differs")
require_contract.call(package.fetch("overrides") == {"@opentelemetry/core"=>"2.10.0", "@opentelemetry/propagator-jaeger"=>"2.9.0", "adm-zip"=>"0.6.0", "uuid"=>"14.0.1"}, "security overrides differ")
require_contract.call(lock.dig("packages", "node_modules/@dagger.io/dagger", "resolved") == "sdk" && lock.dig("packages", "node_modules/@dagger.io/dagger", "link") == true, "lock does not bind local SDK")
require_contract.call(lock.dig("packages", "sdk") == {}, "generated SDK lock entry must not encode a hand-written fixture")

workflows = Dir[File.join(repo, ".github/workflows/*.yml")]
workflows.each do |path|
  workflow = YAML.safe_load(File.read(path), aliases: false)
  workflow.fetch("jobs", {}).values.each do |job|
    runs_on = job["runs-on"]
    next unless runs_on
    labels = runs_on.is_a?(Array) ? runs_on : [runs_on]
    require_contract.call(!labels.include?("hostinger-vps"), "generic self-hosted lane in #{File.basename(path)}")
  end
  steps = workflow.fetch("jobs", {}).values.flat_map { |job| job.fetch("steps", []) }
  steps.map { |step| step["uses"] }.compact.each do |use|
    require_contract.call(use.match?(%r{\A[^/\s]+/[^@\s]+@[0-9a-f]{40}\z}), "mutable action in #{File.basename(path)}: #{use}")
  end
  steps.select { |step| step["uses"]&.start_with?("dagger/dagger-for-github@") }.each do |step|
    require_contract.call(step["with"] == {"version"=>"0.21.8"}, "Dagger action must be installer-only in #{File.basename(path)}")
  end
  steps.select { |step| step["run"].to_s.include?("dagger call") }.each do |step|
    require_contract.call(!step["run"].include?("${{"), "provider expression reached Dagger command in #{File.basename(path)}")
    require_contract.call(step["run"].include?("--payload=.dagger-inputs/"), "Dagger call lacks materialized File input in #{File.basename(path)}")
  end
  steps.each do |step|
    run = step["run"].to_s
    require_contract.call(!run.match?(%r{\b(?:ruby|python(?:3)?|jq|npm|node)\b}), "host scripting runtime in #{File.basename(path)}")
  end
  if steps.any? { |step| step["run"].to_s.include?("dagger call") }
    exact = steps.select { |step| step["run"].to_s.include?("dagger version") }
    require_contract.call(exact.length == 1 && exact.first["run"].include?("= v0.21.8"), "exact Dagger CLI gate differs in #{File.basename(path)}")
  end
end

ci = load_yaml.call(".github/workflows/ci.yml")
trigger = ci["on"] || ci[true]
require_contract.call(trigger.keys.sort == %w[pull_request push workflow_dispatch], "CI triggers differ")
require_contract.call(ci["permissions"] == {"contents"=>"read"}, "CI permissions differ")
ci_steps = ci.dig("jobs", "verify", "steps")
ci_runner = ci.dig("jobs", "verify", "runs-on")
require_contract.call(ci_runner.include?(%q{github.event_name == 'pull_request'}), "CI PR routing missing")
require_contract.call(ci_runner.include?(%q{fromJSON('["self-hosted","Linux","X64","hostinger-vps-pr"]')}), "CI PR runner lane differs")
require_contract.call(ci_runner.include?(%q{fromJSON('["self-hosted","Linux","X64","hostinger-vps-trusted"]')}), "CI protected runner lane differs")
require_contract.call(!ci_runner.include?("github.event.pull_request") && !ci_runner.include?("HOSTINGER_PR_ACTORS"), "PR runner isolation still depends on fork/internal/actor workflow guards")
installer = ci_steps.find { |step| step["uses"]&.start_with?("dagger/dagger-for-github@") }
require_contract.call(installer["if"] == "runner.environment == 'github-hosted'", "hosted installer routing differs")
require_contract.call(ci_steps.any? { |step| step["run"].to_s.include?("materialize-dagger-input.sh ci") }, "CI provider boundary missing")
require_contract.call(ci_steps.any? { |step| step["run"].to_s.include?("dagger call ci --source=. --payload=.dagger-inputs/ci.json") }, "CI Dagger call differs")
dagger_ci = File.read(File.join(repo, "scripts/dagger/ci.sh"))
audit_command = "npm --prefix .dagger audit --package-lock-only --omit=dev --audit-level=high"
executable_lines = dagger_ci.lines.map(&:strip).reject { |line| line.empty? || line.start_with?("#") || line == "set -euo pipefail" }
require_contract.call(executable_lines.first == audit_command && dagger_ci.lines.count { |line| line.strip == audit_command } == 1, "Dagger runtime audit must be first executable command")
preflight = File.read(File.join(repo, "scripts/dagger/preflight-audit.sh"))
require_contract.call(preflight.include?("dagger core --help") && preflight.include?("dagger core container") && preflight.include?("with-directory --path=/src/.dagger --source=.dagger") && preflight.include?("with-exec --args=npm,--prefix,.dagger,audit,--package-lock-only,--omit=dev,--audit-level=high"), "Core container preflight differs")
require_contract.call(preflight.include?("node:22.14.0-bookworm-slim@sha256:1c18d9ab3af4585870b92e4dbc5cac5a0dc77dd13df1a5905cea89fc720eb05b"), "Core preflight image is not pinned")
require_contract.call(!preflight.include?("dagger call") && !preflight.include?(".dagger/src"), "Core preflight must not load project module")
ci_run_steps = ci_steps.each_with_index.map { |step, index| [index, step["run"].to_s] }
preflight_index = ci_run_steps.find { |_, run| run.include?("scripts/dagger/preflight-audit.sh") }&.first
dagger_index = ci_run_steps.find { |_, run| run.include?("dagger call ci --source=. --payload=.dagger-inputs/ci.json") }&.first
require_contract.call(preflight_index && dagger_index && preflight_index < dagger_index, "Core preflight must precede project Dagger module")

materializer = File.read(File.join(repo, "scripts/materialize-dagger-input.sh"))
require_contract.call(materializer.include?('pull_request) cache_namespace=pr') && materializer.include?('push|workflow_dispatch) cache_namespace=trusted') && materializer.include?("*) fail 'unsupported CI event'"), "cache namespace routing must map PR/protected events and fail closed")
require_contract.call(materializer.include?("strict_semver='") && materializer.scan('|[1-9][0-9]*|[0-9]*[A-Za-z-][0-9A-Za-z-]*').length == 2, "strict SemVer prerelease validation differs")
require_contract.call(materializer.scan('must not end with LF').length == 2 && !materializer.match?(/\w+=\$\(required/), "provider env transport must preserve exact values")
source = File.read(File.join(repo, ".dagger/src/index.ts"))
require_contract.call(!source.include?("TrustDomain") && !source.include?("trustDomain") && !source.include?("untrusted"), "workflow trust argument remains in Dagger module")
require_contract.call(source.include?('value !== "trusted" && value !== "pr"') && source.include?('throw new Error("unknown cache namespace")'), "cache namespace validation differs")
require_contract.call(source.include?('make nodejs npm python3 ruby'), "Dagger-owned CI runtimes differ")
%w[go-build go-mod muamba cargo].each do |kind|
  require_contract.call(source.include?("araihu-ci-v1-assets-${cacheNamespace}-#{kind}"), "#{kind} cache lacks stable PR/trusted namespace")
end
require_contract.call(source.scan(".withMountedCache").length == 4, "persistent cache mount set must remain deps/build/tools only")

trusted_lane = ["self-hosted", "Linux", "X64", "hostinger-vps-trusted"]
{
  ".github/workflows/acquisition.yml" => ["acquisition"],
  ".github/workflows/campaigns.yml" => ["resolve"],
  ".github/workflows/release-fanout.yml" => ["dispatch"],
  ".github/workflows/release.yml" => ["publish"],
}.each do |path, jobs|
  workflow = load_yaml.call(path)
  jobs.each { |job| require_contract.call(workflow.dig("jobs", job, "runs-on") == trusted_lane, "protected runner lane differs in #{File.basename(path)}:#{job}") }
end

docs = File.read(File.join(repo, "docs/dagger-ci.md"))
require_contract.call(docs.include?("Cache namespace is an efficiency hint, not a security boundary.") && docs.include?("isolated Engine socket/data") && docs.include?("hostinger-vps-pr") && docs.include?("hostinger-vps-trusted") && docs.include?("preflight-audit.sh") && docs.include?("Dagger Core container"), "runner isolation/cache/preflight documentation missing")

acquisition = load_yaml.call(".github/workflows/acquisition.yml")
acquisition_on = acquisition["on"] || acquisition[true]
require_contract.call(acquisition_on == {"workflow_dispatch"=>nil}, "acquisition must remain manual-only")
puts "CI workflow: exact Dagger, provider input, runner isolation, SDK, and cache contracts valid"
RUBY

forbidden_compact='code''rabbit'
forbidden_spaced='code'' rabbit'
if rg -n -i --glob '!node_modules/**' --glob '!.git/**' "$forbidden_compact|$forbidden_spaced" "$repo"; then
  echo 'retired review automation remains in the repository' >&2
  exit 1
fi
