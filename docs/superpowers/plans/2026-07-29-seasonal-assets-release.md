# Seasonal Assets Release and Activation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Integrate accepted repository batches, publish the Assets/App Shell releases, deploy the complete Ahairu static tree, establish automatic downstream fallback updates, and verify a safe inactive baseline plus an internal campaign.

**Architecture:** The control plane integrates and releases repositories in dependency order. A selected-repository GitHub App supplies short-lived cross-repository dispatch/PR authority. Ahairu alone owns Cloudflare credentials and performs complete Worker deployments. Daily Assets resolution dispatches only when the resolved channel digest changes.

**Tech Stack:** Git, GitHub CLI/API, GitHub Actions, GitHub App authentication, Go 1.26.5, Node 24, Wrangler 4, Cloudflare Workers.

## Global Constraints

- Never push, tag, release, promote, or deploy a batch whose local and combined gates have not passed.
- Never force-push or overwrite an existing tag/release.
- Use selected-repository GitHub App permissions only; do not use an organization-wide classic token.
- Keep Cloudflare secrets only in Ahairu's protected production environment.
- `latest` moves on Assets release; `default` moves only by explicit promotion; `current` moves only by deterministic resolution.
- An unrelated Assets commit cannot change `default` or `current`.
- Initial production deployment has no enabled public campaign.
- Retain `v0.1.0` and every later immutable release in the Worker static tree.
- Channel convergence target is 60 seconds after successful deployment.
- Roll back immediately to the preceding complete Worker version on failed public acceptance; do not retry the same failed promotion blindly.

---

### Task 1: Reconcile and integrate accepted repository branches

**Repositories:** `assets`, `goshtoso`, `goshtoso-app-shells`, `ahairu`, then consumers.

**Files:** No new product files; integration commits only where fast-forward/cherry-pick topology requires them.

**Interfaces:**
- Produces a ledger entry for every accepted commit with exact base, merge order, and disposition.

- [ ] **Step 1: Refresh remotes without pruning and inspect every target**

Record `origin/main`, candidate commits, ahead/behind counts, worktree status, and
open conflicting work. For SSH remotes that fail, use authenticated HTTPS or
GitHub CLI without changing stored credentials or printing tokens.

- [ ] **Step 2: Run fresh reviewer gates on each candidate**

Use one spec-compliance review and one code-quality review per implementation
task. Resolve findings in the owning worktree and rerun focused tests before
acceptance.

- [ ] **Step 3: Integrate Assets first**

Use a fresh integration worktree from current `origin/main`. Apply the approved
spec/plan commit and accepted Assets implementation commits in documented order.
Run the full offline Assets gate. Record the resulting integration SHA.

- [ ] **Step 4: Integrate Goshtoso and App Shells**

Integrate Goshtoso compatibility only if it changes released bytes. Integrate
App Shell commits on current remote main. Run generated-output, test, vet, and
build gates. Record both SHAs.

- [ ] **Step 5: Integrate Ahairu and downstream consumers**

Use the accepted released dependencies, not local replacements. Run each full
gate. Leave Xisnove queued if its active milestone ownership is not resolved;
do not block the baseline Assets/Ahairu release on unrelated dirty work.

- [ ] **Step 6: Push reviewed branches and open PRs**

Push non-force branches. Open PRs with goal, contract changes, exact test
evidence, release dependency, and rollback notes. Wait for required CI/reviews;
do not merge red or stale PRs.

### Task 2: Configure selected-repository GitHub App authority

**Repositories:** GitHub organization settings plus enrolled repositories.

**Files:**
- Modify repository/environment secret configuration, not tracked secret files.
- Modify workflow documentation where secret names are documented.

**Interfaces:**
- App name: `araihu-assets-release-bot` unless already registered.
- Repository access: `assets`, `ahairu`, `goshtoso`, `goshtoso-app-shells`, `goshtoso-charts`, `manja`, `paje`, `xisnove`, and `metaru` only.
- Minimum permissions: metadata read; contents read/write; pull requests read/write; actions read/write only where repository dispatch is used.
- Secret names: `ARAIHU_ASSETS_APP_ID`, `ARAIHU_ASSETS_APP_PRIVATE_KEY`.

- [ ] **Step 1: Audit existing Apps and secret names without exposing values**

Use GitHub API/CLI metadata only. If the named App exists, verify installation
repository selection and permissions. If not, create it through the authenticated
GitHub App manifest/settings flow and require the resulting permission summary
to match this plan before installation.

- [ ] **Step 2: Install on selected repositories only**

Do not choose “all repositories.” Verify the installation repository IDs match
the explicit list. Exclude `fly-deploy`, which has no asset consumer role.

- [ ] **Step 3: Store credentials as scoped organization or repository secrets**

Use secret APIs/CLI stdin. Never print the private key. Restrict organization
secret visibility to the selected repositories. Ahairu Cloudflare credentials
remain separate in its protected `production` environment.

- [ ] **Step 4: Prove token permissions with harmless API calls**

Mint one short-lived token in CI. Read repository metadata and dispatch a
dedicated no-op workflow fixture. Verify denied access to an unselected
repository. Revoke/expire the token normally; never persist it in artifacts.

- [ ] **Step 5: Record non-secret setup evidence**

Document App slug, installation scope, permission names, secret names, and the
successful no-op run URLs. Do not record App private key or token material.

### Task 3: Publish Assets and App Shell releases

**Repositories:** `assets`, `goshtoso-app-shells`; `goshtoso` only if compatibility changes require a release.

**Files:** Release metadata/tags; no ad hoc source edits.

**Interfaces:**
- Assets expected release: `v0.1.1` if catalog patch compatibility passes.
- App Shell version: next SemVer selected from its current release history and public API compatibility.

- [ ] **Step 1: Verify release version availability**

Check local and remote tags and GitHub Releases. Abort on an existing different
tag or artifact. Confirm changelog/comparison links resolve.

- [ ] **Step 2: Rebuild Assets release from the merged commit**

Run the full gate and deterministic double-build. Compare archive and document
hashes with the reviewed release candidate. Require catalog patch compatibility
against `v0.1.0`.

- [ ] **Step 3: Create and push the annotated Assets tag**

Tag the exact merged commit. Push only that tag. Let the release workflow publish
immutable artifacts. Verify GitHub Release checksums and asset names against the
local candidate.

- [ ] **Step 4: Verify `latest` moved while `default` and `current` did not**

Inspect workflow artifacts/state before any Ahairu deployment. A new Assets tag
must not activate presentation.

- [ ] **Step 5: Release App Shells after Assets is immutable**

Run its full clean gate, tag the accepted public API version, push the tag, and
verify consumers can resolve it without local replaces.

- [ ] **Step 6: Release Goshtoso only when bytes/public docs changed**

If Task 1 produced no Goshtoso change, retain released `v0.1.0`. If it did,
select the correct patch version, run root/site gates, tag, and verify the
standalone site pin after the root tag exists.

### Task 4: Deploy the baseline Ahairu static asset service

**Repository:** `ahairu`

**Files:** Deployment artifacts only; source must already be merged.

**Interfaces:**
- `default` selects accepted Assets release and `araihu` theme.
- `current` resolves to that same default because all production campaigns are disabled.

- [ ] **Step 1: Manually promote the default release**

Run the guarded Assets promotion workflow with the exact release and theme.
Verify the produced default/current documents and digest before dispatch.

- [ ] **Step 2: Observe one complete Ahairu deployment**

Require asset artifact hash verification, full site assembly, checks, Wrangler
deploy success, and recorded Worker version. Do not trigger a second deployment
while the first is active.

- [ ] **Step 3: Probe public channels and immutable files**

For `GET` and `HEAD`, verify status, content type, cache control, release/body,
and SHA-256 at:

```text
https://araihu.com/assets/releases/latest
https://araihu.com/assets/releases/default
https://araihu.com/assets/releases/current
https://araihu.com/assets/releases/v0.1.0/catalog.json
https://araihu.com/assets/releases/v0.1.1/release.json
https://araihu.com/assets/campaign/v1.js
```

- [ ] **Step 4: Verify the public brand and license surfaces**

Probe canonical/localized `/brand` and `/license`, sitemap, robots, manifest,
metadata, social previews, immutable downloads, and checksum links. Perform
responsive keyboard/browser checks only through the existing test harness.

- [ ] **Step 5: Verify baseline canary behavior**

No campaign is active. Root source remains `default` without preference and
`preference` with a saved theme. Campaign toggle stays hidden. Failed channel
fetch leaves the rendered baseline unchanged.

- [ ] **Step 6: Record rollback version and retain it**

Identify the immediately preceding Worker version and verify it can be
redeployed. Do not delete it during cleanup.

### Task 5: Add automatic downstream fallback update workflows

**Repositories:** `goshtoso`, `goshtoso-app-shells`, `goshtoso-charts`, `manja`, `paje`, `xisnove`, `metaru` where it owns a fallback.

**Files:**
- Create per consumer: `.github/workflows/araihu-assets-update.yml`
- Create per visual consumer: `scripts/update-araihu-assets.sh` or a repository-native Go updater and focused tests.
- Modify per consumer: integration documentation.

**Interfaces:**
- Trigger: `repository_dispatch` type `araihu-assets-released` plus manual dispatch.
- Payload: release tag, release URL/artifact identity, `release.json` SHA-256.
- Branch naming function: `"automation/araihu-assets-" + releaseTag`, producing `automation/araihu-assets-v0.1.1` for the first run.
- PR labels: `dependencies`, `assets` when labels exist.

- [ ] **Step 1: Write each deterministic updater test first**

Feed a local release fixture. Require catalog-selected files only, exact hashes,
stable ordering, no concepts/review/source files, no network, and a clean second
run. Each repository test names its allowed destination paths.

- [ ] **Step 2: Implement repository-native update commands**

The workflow downloads and verifies the immutable release once. The local
updater copies approved fallback files, updates release/hash constants, runs
generators, and refuses unknown collisions. It does not fetch `current`.

- [ ] **Step 3: Implement guarded PR workflow**

Mint the selected-repository App token. Check out current default branch, run
the updater and repository gates, stop cleanly if no diff, create/update the
versioned automation branch, and open one PR. Never auto-merge.

- [ ] **Step 4: Test dispatch with the current release**

Dispatch every enrolled repository. Require either a clean no-op or one valid PR
with passing CI. Close superseded test PRs only after recording disposition.

- [ ] **Step 5: Add Assets release fan-out**

After immutable release verification, Assets dispatches the enrolled repository
list. A failure in one consumer does not revoke the release or block other
dispatches; it is recorded and retried manually after repair.

### Task 6: Exercise a non-public internal campaign

**Repositories:** `assets`, `ahairu`, canary applications.

**Files:** No permanent production-calendar activation. Use the reviewed disabled `signal-night-proof-2026` entry and a workflow date/enable fixture isolated from public promotion.

**Interfaces:**
- Tests the real resolved channel/runtime/assets without making the campaign the public production `current`.

- [ ] **Step 1: Produce a signed/test channel bundle from the reviewed manifest**

Use a temporary test manifest copy with only `enabled: true` changed and resolver
date `2026-08-01`. Validate, publish to a non-production preview deployment, and
record its digest. Do not commit the enabled change to production main.

- [ ] **Step 2: Run canary browser cases**

Prove baseline first paint, theme CSS preload, atomic theme/brand swap, direct
and sprite toggle rendering, saved preference win, opt-out restoration, reload,
cleared preference, expiry at `2026-08-03`, network failures, reduced motion,
and lifecycle events.

- [ ] **Step 3: Run app-shell and chart canaries**

Verify component-doc and console shells mark sources correctly. Verify Charts
redraws without losing state. Capture only non-sensitive evidence.

- [ ] **Step 4: Tear down only the preview deployment**

Return public production to the unchanged baseline if preview routing touched a
shared route. Keep the production Worker version and release artifacts intact.

- [ ] **Step 5: Record acceptance**

Document preview URL or environment, channel digest, tested browsers/viewports,
results, and any deferred creative work. Do not call the disabled proof campaign
a public seasonal launch.

### Task 7: Final control-plane reconciliation

**Files:** External control ledger and repository release notes.

**Interfaces:**
- Produces terminal disposition for every managed worker, PR, tag, release, deployment, and queued Xisnove task.

- [ ] **Step 1: Reconcile live state against the ledger**

Live repository/CI/deployment state wins. Record mismatches; never revive a
terminal or archived worker from stale ledger text.

- [ ] **Step 2: Run final public probes after 60 seconds**

Require channel bodies and headers to converge. Recheck immutable `v0.1.0` and
new release paths after the latest deployment.

- [ ] **Step 3: Verify every managed session disposition**

Each session is integrated, rejected, superseded, blocked with explicit owner,
or intentionally queued. Archive completed worker sessions after integration
evidence is recorded.

- [ ] **Step 4: Mark the goal complete only when required work is genuinely done**

If Xisnove remains blocked by its active milestone, keep the goal active and
queue its exact adoption task; do not claim all-repository completion. Otherwise
record final tags, deployed URL, PRs, hashes, tests, and rollback version, then
complete the goal.
