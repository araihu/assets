# Runner admission

`cmd/runneradmission` owns Hostinger job admission and maintenance coordination.
Compile with Go 1.27.3 and `CGO_ENABLED=0`; install the executable outside the
runner workspace, owned by root. The GitHub hook must be a `.sh` file, so its
entire adapter is `exec /opt/hostinger-hooks/runneradmission`.

The executable reads runner environment once through `caarlos0/env/v11`.
[Generated environment documentation](../internal/runneradmission/environment.md)
lists required fields and descriptions. Stock envdoc Markdown omits Go field
types; these fields are strings. Regenerate with
`go generate ./internal/runneradmission`. The Go suite checks generated-document
drift using the pinned, vendored envdoc tool.

Root-owned `/etc/hostinger-runner/trust-domain` chooses `trusted` or
`pull-request`. `/etc/hostinger-runner/allowed-actors` retains the human allowlist.
Existing human, Dependabot, release, and fly-deploy event rules are preserved.
The Assets App is never added to that global allowlist.

`arai-hu-assets-distribution[bot]` may dispatch `araihu-assets-released` only
to the five enrolled consumers on `refs/heads/main`. The event must contain
exactly the six immutable release fields with valid canonical URL, hashes,
revision, and stable tag. Consumers still verify the tag, archive bytes and
catalog before writing files. Admission validates event shape, not provenance
over the network.

In the PR lane, the App may test its same-repository, non-draft asset updates
targeting `main`, with head `automation/araihu-assets-vX.Y.Z`. Human reruns of
those PRs are allowed. Forks and unrelated App branches remain denied. Bot
pushes, manual workflows, releases, and trusted execution of PR code are denied.

After policy passes, the tool holds `.maintenance.lock`, checks the infrastructure
maintenance lock and marker, then creates `.job-active`. An active maintenance
operation denies admission. A stale marker is removed only after acquiring its
infrastructure lock. Denials do not print the provider payload or create the
job marker. Errors fail the job before workflow steps run.

Tests cover policy mutations, missing and malformed context, bounded payloads,
duplicate JSON fields, lane isolation, stale markers and active maintenance.
