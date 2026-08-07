# Provenance schema

The generated provenance files explain where each distributed icon came from.
They are deterministic traceability records. Release integrity remains the
responsibility of `SHA256SUMS`, the extracted `checksums.txt`, and
`dist/release.json`.

## Files and top-level fields

The release contains one provenance file for each attributed source family:

- `dist/provenance/heroicons.json`
- `dist/provenance/developer-icons.json`

Each file records the source family, upstream release, revision, repository,
archive URL, locked archive SHA-384 integrity, selected variants, and asset
count. It also records the distributed MIT license path for the source family.
The release inventory hashes each provenance file as a normal release member.

## Per-asset records

Every asset record contains its exact catalog `canonicalName`, safe
`spriteSymbol` when the asset is included in a sprite, upstream source path,
distributed release path, and the SHA-256 of the normalized SVG bytes. The
normalized-artifact hash is intentionally different from an upstream source
archive hash: it identifies the bytes that a consumer actually renders.

Preserve `canonicalName` and `spriteSymbol` exactly, including mixed-case names
such as `brand-developer-icons-tRPC`; only a consumer's programming-language
identifier may be normalized. A provenance record does not authorize a consumer
to alter protected brand colors or imply trademark permission.

## Verification order

1. Verify the GitHub Release archive against its `SHA256SUMS` record.
2. Extract into a new empty directory and verify every member with
   `checksums.txt`.
3. Verify `release.json` inventory and catalog references.
4. Use provenance for source and license traceability while preserving the
   distributed `NOTICE` and upstream MIT license files.

Provenance must not be used as a substitute for any of those digest checks, and
it does not authenticate an upstream publisher or grant trademark rights.
