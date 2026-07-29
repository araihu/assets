# Arai Hû Assets v0.1.0 core release candidate checklist

## Scope and status

This is an **untagged core release candidate**, not an immutable final release.
It freezes the catalog, sprites, generated proof, and archives consumed by
downstream work. No tag, push, merge, deployment, or Goshtoso work is
authorized by this checklist.

## Frozen core inputs

- [x] Generate the core RC commit from this checklist and generated output; its
  exact commit is recorded in the ignored Task 10 report after commit.
- [x] `dist/catalog.json`: `d83be964fa411e87c61b49f0a0b6a2a1465f33ad43bea7cd93b2e434b59266af`.
- [x] `dist/icons/brand/sprite.svg`: `249bad81cff862871be29b2e79eb903aa5c3fc832ba464a81f42134c310c1b6f`.
- [x] `dist/icons/ui/sprite.svg`: `6b312ee2cf9f0e91c4621bd4eec348ecaf39cfdc0a00c8ddefdf4d7f8e9f32a5`.
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
- [ ] G1 must prove Goshtoso catalog/sprite compatibility from this frozen core input.
- [ ] W1 must prove public-site compatibility after G1.
- [ ] Root-owned final review must validate proof, consumer compatibility, final archives,
  then obtain explicit user approval before tag, push, merge, or deployment.

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
- `dist/icons/brand/sprite.svg`: `249bad81cff862871be29b2e79eb903aa5c3fc832ba464a81f42134c310c1b6f`.
- `dist/icons/ui/sprite.svg`: `6b312ee2cf9f0e91c4621bd4eec348ecaf39cfdc0a00c8ddefdf4d7f8e9f32a5`.

- `dist/releases/araihu-assets-v0.1.0.tar.gz`:
  `be7cc2c3b551d9825ea47a04935adba903dc1fe7e05ea454540cbd3f61e03034`
- `dist/releases/araihu-assets-v0.1.0.zip`:
  `20669e0c4ffa50f4caca26e9bc3310cc2732f70ea8a9a51f0db56a38f9ed64a6`

CI bootstrap provenance:

- cargo-c source `v0.10.10`:
  `da2101c5bee6c4bc0d62785c7b79d74a22dd566f93f0530b70d82531d4340b80`
- cargo-c `v0.10.10` release `Cargo.lock`:
  `3d9107cb39d4d3c3503eed03fd668f8c24ad94d2a836f7e8c31f782c31b4a548`
