# Catalog schema v2

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
  "schemaVersion": 2,
  "release": "v0.2.3",
  "identityRevision": 11,
  "assets": []
}
```

- `schemaVersion` is exactly `2` for new releases. Decoders retain schema v1
  support for immutable historical catalogs.
- `release` is a `vMAJOR.MINOR.PATCH` release tag, optionally with normal
  SemVer prerelease/build suffixes.
- `identityRevision` is exactly `11`; public paths still omit
  `v11`.
- `assets` is non-empty and contains unique `canonicalName` values.

## Asset object

Every asset carries these required camel-case fields:

```text
canonicalName namespace path product artwork appearance surface framing
format dimensions spriteSymbol colorBehavior license source sha256
```

`product`, `artwork`, `surface`, and `framing` are lower-kebab identifiers.
`appearance` is a lower-kebab variant that may start with a digit, such as
`16-solid`. `namespace` is `brand` or `ui`. Schema v2 canonical names preserve
literal ASCII case within kebab-separated segments, so
`brand-developer-icons-tRPC` is valid and must not be silently lowercased.
Schema v1 canonical names remain lower-kebab.

`path` is relative to `dist/`, with no `dist/` prefix, absolute path, traversal,
or source-tree location. It must start with `brand/`, `icons/`, or `platform/`,
and its extension must match `format`. Schema v2 supports `svg` and `png`.
Thus `icons/ui/heroicons/16-solid-check.svg` is valid; neither
`dist/icons/ui/heroicons/16-solid-check.svg` nor
`source/brand/original/check.svg` is valid.

`dimensions` has optional positive `width` and `height` (they occur together)
and optional `viewBox`. SVG entries require a four-number `viewBox` with
positive width and height. PNG entries require width and height and omit
`viewBox`.

SVG entries may have a unique lower-kebab `spriteSymbol`. It is an independent,
downstream-safe identifier rather than a derivation rule; for example,
`brand-developer-icons-tRPC` declares `devicon-trpc`. An empty value means
the SVG is available only through its individual `path` and is not present in a
sprite; consumers must not synthesize a symbol name. PNG entries always have an
empty `spriteSymbol`. `colorBehavior` is one of `protected`, `monochrome`, or
`tintable`. Only `monochrome` and `tintable` entries may inherit `currentColor`;
`protected` entries must retain their designed colors.

`license` and `source` are non-empty single-line provenance labels. `sha256` is
the lowercase hexadecimal SHA-256 of the published artifact.

Catalog emission sorts assets by `canonicalName`, then `path`, uses two-space
JSON indentation, and terminates with one newline. Decoders reject unknown,
duplicate, and case-variant keys at every schema object, plus more than one JSON
value.

## Compatibility

Changing from schema v1 to schema v2 is a minor-version contract change. Within
one schema version, adding a canonical icon is compatible only when every
existing canonical name remains present with identical namespace, product,
artwork, appearance, surface, framing, format, dimensions, sprite symbol,
color behavior, license, and source. Removal, rename, or any such semantic
change is not patch-compatible. Artifact path and SHA-256 may change only when
those preserved semantics remain true; consumers must use the released catalog
rather than hard-code paths or hashes.
