# Goshtoso integration

Goshtoso stays brand-neutral. Read a validated `dist/catalog.json`, select
entries by canonical name and namespace, and use the catalog's exact
`spriteSymbol`; never derive a symbol from a filename.

Use the public `v0.2.1` [immutable Git tag](https://github.com/araihu/assets/tree/v0.2.1),
[GitHub Release](https://github.com/araihu/assets/releases/tag/v0.2.1),
[tar archive](https://github.com/araihu/assets/releases/download/v0.2.1/araihu-assets-v0.2.1.tar.gz),
and [SHA256SUMS](https://github.com/araihu/assets/releases/download/v0.2.1/SHA256SUMS).
Before adopting its files, require matching release values in `catalog.json`,
`themes.json`, and `release.json`, then verify the inventory hashes in
`release.json`. `v0.1.1` remains the published promoted default and `v0.2.0`
is the preceding release.

For the published release, download the immutable tar archive and its
`SHA256SUMS` file into a disposable directory. Check the archive digest, extract
into a new empty release root, then run `sha256sum --check --strict
checksums.txt` from that root before passing it to Goshtoso. The archive URL,
release value, the selected archive's own SHA-256, and the catalog,
`release.json`, and `checksums.txt` SHA-256 values belong to the Arai Hû Assets
release, not to Goshtoso. Tar and zip containers are not byte-equivalent;
verify the digest for the format you downloaded, then use the extracted,
member-equivalent release root as the input.

Its generic sprite component owns sprite URL or inline mode, symbol, size,
accessible label, decorative state, CSS classes, and `currentColor` for
compatible UI icons. Arai Hû Assets owns brand geometry, recipes, designed
colors, platform padding, provenance, licenses, checksums, and catalog data.

The explicit Goshtoso input is either the verified release root extracted from
the published tar or zip archive, or the verified `dist/` root itself. Verify
the selected archive against its own published checksum, then verify
`checksums.txt`, `release.json`, and every selected catalog artifact before
generating bindings. Never point generation at this repository's
`internal/acquisition/vendor/` tree.

Schema v2 consumers preserve `canonicalName`, `path`, and `spriteSymbol`
literally. The mixed-case name `brand-developer-icons-tRPC` is selected by that
exact string and renders with the catalog-declared `devicon-trpc` symbol; Go
identifier normalization is a separate generated concern. Consumers must keep
`NOTICE`, license files, and provenance beside the generated package.

The assets CLI does not generate Go source. Goshtoso may generate its own typed
Heroicons names and project-local brand bindings from a schema-v2 catalog.
Arai Hû-specific names must not enter Goshtoso's public package.
