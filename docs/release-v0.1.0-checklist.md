# Arai Hû Assets v0.1.0 release checklist

## Scope and status

This checklist records the catalog, sprites, generated proof, archives, and
cross-repository gates approved for the immutable `v0.1.0` release.

## Frozen core inputs

- [x] Generate the core RC commit from this checklist and generated output; its
  exact commit is recorded in the ignored Task 10 report after commit.
- [x] `dist/catalog.json`: `d83be964fa411e87c61b49f0a0b6a2a1465f33ad43bea7cd93b2e434b59266af`.
- [x] `dist/icons/brand/sprite.svg`: `e0c98a783cf65cf52b0a57cca47b84704499200a7fdb113b751d8f6c5828ba45`.
- [x] `dist/icons/ui/sprite.svg`: `75e282de7a19efba9cf0285b44af0641c1527361f921b7d7f8020efc1f1f0fb7`.
- [x] Catalog: 302 assets (235 brand, 67 UI). Pre-P1 core `dist/`: 352 regular files.

## Completed core gates

- [x] `make generate` replaced the legacy managed `dist/` tree.
- [x] Offline `make check`, `make verify`, `make release`, full tests, and vet pass.
- [x] Catalog command validates and reports the frozen catalog.
- [x] Pre-P1 archive members inspected (350 in each archive); reproducibility rebuild left no diff.
- [x] CI selects Go `1.26.5`, asserts its exact version, builds checksum-verified
  `rsvg-convert 2.62.1`, and runs Go/build gates with `GOPROXY=off`.
- [x] CI never runs `make vendor`; only its separate, checksum-verified renderer
  bootstrap uses network access.
- [x] CI pins `actions/setup-go v6.0.0` to
  `44694675825211faa026b3c33043df3e48a5fa00`; it builds
  `cargo-cbuild 0.10.10+cargo-0.86.0` from checksum-verified cargo-c source
  plus its checksum-verified release `Cargo.lock`, with `cargo --locked`.

## Explicit P1 and cross-repository gates

- [x] P1 replaced legacy V11 proof with catalog-driven `dist/proof/**`.
- [x] P1 added offline `proof --check` validation and archive inclusion for `dist/proof/**`.
- [x] P1 regenerated both release archives and recorded final P1 SHA-256 values.
- [x] P1 enabled `make proof-check` in CI and local release verification.
- [x] G1 proved Goshtoso catalog/sprite compatibility from this frozen core input.
- [x] W1 proved public-site compatibility after G1.
- [x] Root-owned final review validated proof, consumer compatibility, and final archives.
- [x] The user explicitly approved tag, push, merge, and deployment on 2026-07-29.

## Final P1 archive rule

The current `dist/releases/araihu-assets-v0.1.0.tar.gz` and `.zip` archives
contain every managed non-archive file, including all 351 files under
`dist/proof/**`; each has 701 members. They exclude review screenshots and
critique files. `make proof` and `make proof-check` are offline and invoke
`araihu-assets proof [--check]`; neither invokes legacy V11 scripts.

## Core RC evidence

Task 10 records exact commands, tool versions, counts, hashes, archive
inspection, reproducibility evidence, commit, and pending gates in its ignored
handoff report.

Generated locally with `go version go1.26.5 darwin/arm64` and
`rsvg-convert version 2.62.1`. Final P1 artifact values:

- Managed `dist/`: 703 regular files; `dist/proof/`: 351 regular files.
- `dist/catalog.json`: `d83be964fa411e87c61b49f0a0b6a2a1465f33ad43bea7cd93b2e434b59266af`.
- `dist/icons/brand/sprite.svg`: `e0c98a783cf65cf52b0a57cca47b84704499200a7fdb113b751d8f6c5828ba45`.
- `dist/icons/ui/sprite.svg`: `75e282de7a19efba9cf0285b44af0641c1527361f921b7d7f8020efc1f1f0fb7`.

- `dist/releases/araihu-assets-v0.1.0.tar.gz`:
  `7097178a62fb0dfdbaca9490259b71c0ccb4294e52a9ee3df73f3b2fda588286`
- `dist/releases/araihu-assets-v0.1.0.zip`:
  `914993a9a84b63dbe02dc9790c66bb29c789b31aa247f0c3195712a255c55d0c`

CI bootstrap provenance:

- cargo-c source `v0.10.10`:
  `da2101c5bee6c4bc0d62785c7b79d74a22dd566f93f0530b70d82531d4340b80`
- cargo-c `v0.10.10` release `Cargo.lock`:
  `3d9107cb39d4d3c3503eed03fd668f8c24ad94d2a836f7e8c31f782c31b4a548`
