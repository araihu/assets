# Arai Hû Assets v0.1.1 release-candidate checklist

## Status

`v0.1.1` is generated and locally verified as an offline release candidate.
It has no tag, push, GitHub release, public archive, promotion, or deployment.
Integration review must accept the immutable bundle before any publication step.

## Authoritative release identity

- [x] `internal/releaseinfo.Version` is the sole release identity: `v0.1.1`.
- [x] Catalog, theme catalog, release metadata, NOTICE, checksums (through the
  hashed release metadata), and archive names consume that identity.
- [x] `TestRunEmitsConsistentV011ReleaseFields` proved RED against the former
  `v0.1.0` output, then GREEN after the single-source implementation. It checks
  catalog, themes, release metadata, checksums, and both archive names.

## Generated immutable bundle

- [x] Offline `make generate` published catalog, themes, campaigns, runtime,
  release metadata, checksums, sprites, platform assets, proof, and archives.
- [x] Current catalog: schema v1, release `v0.1.1`, identity revision 11,
  302 assets (235 brand, 67 UI).
- [x] `release.json`: schema v1, runtime version 1, 710 inventoried files.
- [x] `releases/v0.1.0/` retains the four captured immutable channel inputs
  (`release.json`, `catalog.json`, `themes.json`, `campaigns.json`) from this
  checkout's v0.1.0 history. Default/current remain stable on v0.1.0 while a
  later release only advances `latest`.

Archive SHA-256:

- `araihu-assets-v0.1.1.tar.gz`:
  `4f9549a5975a284921fa7eb263cbd1a92c065d323ab395e82c3eac1d0a46886b`
- `araihu-assets-v0.1.1.zip`:
  `18d2e1e0fdf0e22fbf2e32c803c7ffe16008b3a2aebc6d2d8bdab03ce65c55be`

## Patch compatibility with v0.1.0

- [x] Decoded `v0.1.0` catalog from Git tag and candidate catalog.
- [x] 302 prior canonical assets retained; missing: 0; changed full asset or
  semantic fields: 0; additions: 0.
- [x] Release-document changes: `release`, `catalogSha256`, and
  `themesSha256`. `schemaVersion`, `identityRevision`, `runtimeVersion`,
  `campaignsSha256`, and inventory count (710) remain unchanged.

## Deterministic channel proof

- [x] Ran `campaigns publish --date 2026-10-31` twice into separate `mktemp -d`
  outputs. Recursive SHA-256 manifests matched exactly: four files.
- [x] Output hashes: `campaign/v1.js`
  `a936193b4fed8120e6cb3423f19d3e2ddb0ba32266dc4e5f02a98f5261853709`;
  `releases/default.json` and `releases/current.json`
  `670c7e613eb7b15b7110f7fbc95f923193b1e5c3259ce048c2c8e5323ae4ae07`;
  `releases/latest.json`
  `8bab7b3817378b5aa2aa6512fa70f536ec888c289dbc2d9f9e09bcf62d56db0d`.
- [x] Both validated temporary directories were removed after comparison.

## Gates and technical debt

- [x] RED: `go test ./internal/build -run '^TestRunEmitsConsistentV011ReleaseFields$' -count=1`
  failed as expected: catalog release was `v0.1.0`, wanted `v0.1.1`.
- [x] GREEN: same focused command passed; `go test ./internal/build -count=1`
  passed after generation.
- [x] `make release && git diff --check` passed. `make release` ran full
  `go test ./... -count=1`, deterministic offline build check, proof check,
  and catalog validation.
- [x] Focused post-snapshot checks: `go test ./internal/app ./internal/channels -count=1` passed.
- [ ] Deferred by delivery cadence: standalone `make proof`, `make check`,
  `make proof-check`, and `go test ./...` (all already covered by `make release`);
  standalone `make verify` and `go vet ./...` remain unrun. Re-run both before
  tag/publication if integration policy requires independent coverage.

## Stop condition

No tag, push, release, publication, promotion, or deployment was performed.
