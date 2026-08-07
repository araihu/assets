# Arai Hû Assets v0.2.0 release checklist

This is a frozen local candidate record. It does not claim a tag, push, GitHub
Release, publication, deployment, or consumer integration.

## Acquisition boundary

- [x] `.muamba.yaml` declares Heroicons `v2.2.0` and Developer Icons `v7.0.1`
  through bounded HTTPS tar.gz directory sources.
- [x] `.muamba.lock.yaml` SHA-256:
  `0e018778ec5236bdf5af39c33fcb6291d7e2c307322f78df8e48f41543054b16`.
- [x] The lock resolves exactly 1,288 Heroicons SVGs across `16/solid`,
  `20/solid`, `24/outline`, and `24/solid`, plus 318 individual Developer Icons
  SVGs. The upstream aggregate Developer Icons sprite is excluded.
- [x] Muamba prerequisite consumed from the public `v0.0.4` release commit
  `eb5b5cdc59c0a7ed8a9dde85597005a427870b57`, tree
  `3e5fcd296e578318bbed43e6fa94eab8394f7c5f`.
- [x] Muamba `v0.0.4` is published; `go.mod`, `go.sum`, and `vendor/` pin and
  materialize the public module, while the icon acquisition inputs and their
  generated outputs remain unchanged.

## Catalog and generated artifacts

- [x] Catalog schema v2 explicitly preserves mixed-case canonical-name
  segments and permits size-prefixed appearance variants. Historical schema v1
  catalogs remain readable with their lower-kebab canonical-name rule.
- [x] `dist/catalog.json`: 1,841 assets total; 1,288 Heroicons and 318 Developer
  Icons; SHA-256
  `a0e8e5c8928e37de979ce9a60f3d66fad1aa1b4c7d2904f9275f0be9932a33d6`.
- [x] Literal `brand-developer-icons-tRPC` is preserved while its independent
  sprite symbol is `devicon-trpc`.
- [x] Heroicons sprite SHA-256:
  `65cdb814125787460b548428dd49edd8e29250ee9eba5e6f27f4eb1b746fc3ca`.
- [x] Developer Icons sprite SHA-256:
  `be3862d97ca02f2c386d7bb73174290d76ae68b27ff45c8f716d219f8f513a55`.
- [x] Heroicons provenance SHA-256:
  `41f649e87d00f2c11f3a55bcb024d15c596f3938ec325f38f4433124edc059c6`.
- [x] Developer Icons provenance SHA-256:
  `67b4f6e45ef2bd7778c6b5da108a61ea6f49e4a42808535b4318850d213367f3`.
- [x] Every locked source member is size- and SHA-384-verified before SVG
  normalization; generated SVG and sprite validators reject unsafe content.

## Release and consumer boundary

- [x] `dist/release.json` SHA-256:
  `77c696ae5eceb5e7bc11d19affb7c2c7b7e8afc6414882b9b059239e315f2260`.
- [x] `dist/checksums.txt` SHA-256:
  `334005c77622250a1e827b9472161cd6e56c82d487fc0d44023d49261f8dbee5`.
- [x] Tar archive: 3,797 members; SHA-256
  `5d7d691e22d4071507b0bf2248713d7008adf57c18840cfd46e20901db0b78e5`.
- [x] Zip archive: 3,797 members; SHA-256
  `881094d3d161b79904fcfad320c26d947c9a1e526ee0b69ce8a2d04c3ff4b1b0`.
- [x] Both archives extract cleanly and all 3,796 records in their embedded
  `checksums.txt` verify.
- [x] Goshtoso's later input is the verified root extracted from either
  `araihu-assets-v0.2.0.tar.gz` or the zip archive, using that format's own
  archive digest; the extracted members are equivalent, but the containers are
  not byte-equivalent. It is never the vendored source tree. The assets CLI
  emits no Go or other client-language bindings.
- [x] The generated proof has initial-HTML canonical and `og:url` values that
  resolve to the actual `dist/proof/index.html` document, plus description,
  Open Graph, and X-card metadata and a validated 1280x640 PNG below 1 MiB.

## Gates

- [x] Muamba strict verify and generated-Go drift check.
- [x] `make check` and deterministic offline build check.
- [x] `make proof-check`.
- [x] `go test ./... -count=1`.
- [x] `go vet ./...`.
- [x] `make verify`.
- [x] `make release`.
- [x] `go test -race ./... -count=1`.
- [x] CI and release-workflow guard scripts.
- [x] `git diff --check`.

## Documentation gate

- [x] README distinguishes source-checkout reproducibility checks from
  published archive and extracted-member verification.
- [x] Consumer guides document `SHA256SUMS`, `checksums.txt`, schema-v2
  migration, exact mixed-case canonical names, and literal sprite symbols.
- [x] Provenance fields and their traceability-only role are documented in
  [`docs/provenance-schema.md`](provenance-schema.md).
- [x] Candidate status remains explicit: no public tag, archive, or module
  version is advertised until the release is published.
- [x] Publish the Muamba release, replace the unpublished pseudo-version, and
  rerun the full release/archive/checksum gate before tagging Assets.
