# Goshtoso integration

Goshtoso stays brand-neutral. Read a validated `dist/catalog.json`, select
entries by canonical name and namespace, and use the catalog's exact
`spriteSymbol`; never derive a symbol from a filename.

Before adopting this checkout's `v0.2.0` candidate, require matching release
values in `catalog.json`, `themes.json`, and `release.json`, then verify the
inventory hashes in `release.json`. `v0.1.1` remains the published promoted
default and `v0.1.3` remains the latest published release. `v0.2.0` stays local
and offline until integration review accepts it and publishes its tag and
GitHub Release.

Its generic sprite component owns sprite URL or inline mode, symbol, size,
accessible label, decorative state, CSS classes, and `currentColor` for
compatible UI icons. Arai Hû Assets owns brand geometry, recipes, designed
colors, platform padding, provenance, licenses, checksums, and catalog data.

The explicit Goshtoso input is either the verified release root extracted from
`dist/releases/araihu-assets-v0.2.0.tar.gz` (or the byte-equivalent zip), or the
verified `dist/` root itself. Verify the archive against its published release
checksum, then verify `checksums.txt`, `release.json`, and every selected catalog
artifact before generating bindings. Never point generation at this repository's
`internal/acquisition/vendor/` tree.

The assets CLI does not generate Go source. Goshtoso may generate its own typed
Heroicons names and project-local brand bindings from a schema-v2 catalog.
Arai Hû-specific names must not enter Goshtoso's public package.
