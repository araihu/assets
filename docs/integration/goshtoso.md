# Goshtoso integration

Goshtoso stays brand-neutral. Read a validated `dist/catalog.json`, select
entries by canonical name and namespace, and use the catalog's exact
`spriteSymbol`; never derive a symbol from a filename.

Its generic sprite component owns sprite URL or inline mode, symbol, size,
accessible label, decorative state, CSS classes, and `currentColor` for
compatible UI icons. Arai Hû Assets owns brand geometry, recipes, designed
colors, platform padding, provenance, licenses, checksums, and catalog data.

The assets CLI does not generate Go source. Goshtoso may generate its own typed
Heroicons names and project-local brand bindings from a schema-v1 catalog.
Arai Hû-specific names must not enter Goshtoso's public package.
