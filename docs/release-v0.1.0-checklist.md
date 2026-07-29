# Arai Hû Assets v0.1.0 core release candidate checklist

## Scope and status

This is an **untagged core release candidate**, not an immutable final release.
It freezes the catalog and sprite inputs consumed by P1 and Goshtoso. No tag,
push, merge, deployment, proof implementation, or Goshtoso work is authorized
by this checklist.

## Frozen core inputs

- [x] Generate the core RC commit from this checklist and generated output; its
  exact commit is recorded in the ignored Task 10 report after commit.
- [x] `dist/catalog.json`: `d83be964fa411e87c61b49f0a0b6a2a1465f33ad43bea7cd93b2e434b59266af`.
- [x] `dist/icons/brand/sprite.svg`: `0fed5e1b99457f812c5d945b7f59b97a445dd9f32d0f5f99cc35c97666063fe6`.
- [x] `dist/icons/ui/sprite.svg`: `ee25f8d3a9abfc4fbbbd1d06c8a53049fbfb59907545019d7a47a64961fc6cf7`.
- [x] Catalog: 302 assets (235 brand, 67 UI). Core `dist/`: 352 regular files.

## Completed core gates

- [x] `make generate` replaced the legacy managed `dist/` tree.
- [x] Offline `make check`, `make verify`, `make release`, full tests, and vet pass.
- [x] Catalog command validates and reports the frozen catalog.
- [x] Archive members inspected (350 in each archive); reproducibility rebuild leaves no diff.
- [x] CI selects Go `1.26.5`, asserts its exact version, builds checksum-verified
  `rsvg-convert 2.62.1`, and runs Go/build gates with `GOPROXY=off`.
- [x] CI never runs `make vendor`; only its separate, checksum-verified renderer
  bootstrap uses network access.

## Explicit P1 and cross-repository gates

- [ ] P1 must replace legacy V11 proof with catalog-driven `dist/proof/**`.
- [ ] P1 must add proof check-mode validation and archive inclusion for `dist/proof/**`.
- [ ] P1 must regenerate both release archives and record **final** archive SHA-256 values.
- [ ] P1 must enable `make proof-check` in CI and final release verification.
- [ ] G1 must prove Goshtoso catalog/sprite compatibility from this frozen core input.
- [ ] W1 must prove public-site compatibility after G1.
- [ ] Root-owned final review must validate proof, consumer compatibility, final archives,
  then obtain explicit user approval before tag, push, merge, or deployment.

## Provisional archive rule

The current `dist/releases/araihu-assets-v0.1.0.tar.gz` and `.zip` hashes are
**provisional**. P1 adds `dist/proof/**`, so current archives intentionally do
not meet final-release membership. Do not call legacy `make proof` after
managed `dist/` generation: it depends on removed `dist/v11`; P1 owns its
replacement. Final archive verification is pending P1.

## Core RC evidence

Task 10 records exact commands, tool versions, counts, hashes, archive
inspection, reproducibility evidence, commit, and pending gates in its ignored
handoff report.

Generated locally with `go version go1.26.5 darwin/arm64` and
`rsvg-convert version 2.62.1`. Provisional archive values, before P1:

- `dist/releases/araihu-assets-v0.1.0.tar.gz`:
  `698e06cbc577189d8d7f440c066eae5f08fe31c105abbcb499ecdcde534436f2`
- `dist/releases/araihu-assets-v0.1.0.zip`:
  `4195f7707c15c936bb09892944783bf0b9ea323029d4ab3ef609f8b68cceb588`
