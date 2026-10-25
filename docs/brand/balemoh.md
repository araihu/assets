# Balemoh identity

Balemoh is `homelab` spelled backwards. Its mark is a solid house rotated
180 degrees, including the doorway and chimney. The roof and chimney point down and
form one continuous shape in the Arai Hû signal color. No terminal chevron remains.

The source geometry uses two paths: the house silhouette with an open doorway,
and the roof with its integrated chimney. Monochrome variants merge their appearance through the
existing generator's `currentColor` contract.

## Palette and sources

The brand uses the semantic palette from `manifests/brand.yaml`:

| Role | Light | Dark |
| --- | --- | --- |
| House and wordmark, ink | `#07111f` | `#f3f2e9` |
| Roof and chimney, signal | `#c7ff4a` | `#c7ff4a` |
| Plated background, surface | `#f3f2e9` | `#07111f` |

Four promoted SVG masters live under `source/brand/original/balemoh-*.svg`.
The wordmark preserves the outlined Arial Rounded MT Bold lettering from the
earlier Balemoh concept. It contains paths, not embedded fonts or live text.
Source hashes are pinned by the product's manifest entry.

## Distribution

Balemoh uses the same recipes, normalization, sprites, catalog, checksums,
platform packaging and proof scenarios as the other Arai Hû products:

- `icons/brand/balemoh-icon-*.svg` contains optical and launcher marks.
- `brand/balemoh/logo/*.svg` contains horizontal wordmarks.
- `platform/web/balemoh/` contains favicon, touch icon and PWA files.
- `platform/android/balemoh/` contains legacy and adaptive launcher resources.
- `platform/apple/balemoh/` contains the light, dark and tinted app icon set.

Resolve consumer paths and hashes through `catalog.json` in a release or a
verified local build. Do not import from the original source paths.

`make generate`, `make check`, `make proof-check` and `make verify` use the
repository's renderer pin. Adding this product locally does not publish a new
release or change any consumer's pinned dependency. Distribution requires a
subsequent authorized release.
