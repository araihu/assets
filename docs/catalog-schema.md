# Catalog schema v1

`dist/catalog.json` is Arai Hu Assets' language-neutral release contract. It
contains generated, distributable visual artifacts only. It never lists working
sources, manifests, vendored inputs, concepts, documentation, or files outside
`dist/`.

Consumers read this JSON and own any bindings they generate. In particular,
`araihu-assets` does not generate Go, TypeScript, or another client language's
source. Goshtoso and other clients may generate project-local bindings from a
validated catalog.

## Top-level object

```json
{
  "schemaVersion": 1,
  "release": "v0.1.0",
  "identityRevision": 11,
  "assets": []
}
```

- `schemaVersion` is exactly `1`.
- `release` is a `vMAJOR.MINOR.PATCH` release tag, optionally with normal
  SemVer prerelease/build suffixes.
- `identityRevision` is a positive traceability value; v0.1.0 records `11`.
- `assets` is non-empty and contains unique `canonicalName` values.

## Asset object

Every asset carries these required camel-case fields:

```text
canonicalName namespace path product artwork appearance surface framing
format dimensions spriteSymbol colorBehavior license source sha256
```

`canonicalName`, `product`, `artwork`, `appearance`, `surface`, and `framing`
are lower-kebab identifiers. `namespace` is `brand` or `ui`.

`path` is relative to `dist/`, with no `dist/` prefix, absolute path, traversal,
or source-tree location. It must start with `brand/`, `icons/`, or `platform/`,
and its extension must match `format`. Schema v1 supports `svg` and `png`.
Thus `icons/ui/heroicons/16-solid-check.svg` is valid; neither
`dist/icons/ui/heroicons/16-solid-check.svg` nor
`source/brand/original/check.svg` is valid.

`dimensions` has optional positive `width` and `height` (they occur together)
and optional `viewBox`. SVG entries require a four-number `viewBox` with
positive width and height. PNG entries require width and height and omit
`viewBox`.

SVG entries require a unique lower-kebab `spriteSymbol`; PNG entries have an
empty `spriteSymbol`. `colorBehavior` is one of `protected`, `monochrome`, or
`tintable`. Only `monochrome` and `tintable` entries may inherit `currentColor`;
`protected` entries must retain their designed colors.

`license` and `source` are non-empty single-line provenance labels. `sha256` is
the lowercase hexadecimal SHA-256 of the published artifact.

Catalog emission sorts assets by `canonicalName`, then `path`, uses two-space
JSON indentation, and terminates with one newline. Decoders reject unknown
fields and more than one JSON value.

## Patch compatibility

For schema v1, adding a canonical icon is patch-compatible only when every
existing canonical name remains present with identical namespace, product,
artwork, appearance, surface, framing, format, dimensions, sprite symbol,
color behavior, license, and source. Removal, rename, or any such semantic
change is not patch-compatible. Artifact path and SHA-256 may change only when
those preserved semantics remain true; consumers must use the released catalog
rather than hard-code paths or hashes.
