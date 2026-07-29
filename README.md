# Arai Hû Assets

Deterministic, release-ready identity and interface assets for Arai Hû
products. After Task 10, the release contract is `dist/catalog.json`; consumers
use its declared paths, symbols, licenses, and SHA-256 values rather than
source-tree paths or an identity working revision.

## Current checkout

Requires Go `1.26.5`. A supported older local Go installation may select that
toolchain with `GOTOOLCHAIN=auto`.

Before Task 10 creates the release candidate, this repository intentionally
does not contain a published `v0.1.0` catalog or archive. Use a disposable
checkout when generating the managed `dist/` tree:

```sh
make generate
go run ./cmd/araihu-assets catalog
make check
make verify
```

Run `catalog` only after a successful `make generate`, because it validates the
generated `dist/catalog.json`. In the pre-RC tracking checkout, `make check`
correctly reports drift instead of replacing the retained V11 proof scaffold.

## Install after release

No `v0.1.0` tag exists yet. After an explicit tag and release are approved,
install and verify the released module:

```sh
go install github.com/araihu/assets/cmd/araihu-assets@v0.1.0
araihu-assets verify
```

For release-maintenance work, stable Make targets are:

```sh
make vendor    # fetch only locked manifest-selected UI sources
make generate  # build offline and atomically replace managed dist/
make verify    # rebuild offline and compare with dist/
make check     # test, then reject generated-output drift
make proof-check
```

The current `make proof-check` remains a V11 calibration gate until P1 replaces
it with the catalog-driven proof site.

## CLI

```text
araihu-assets vendor
araihu-assets build --offline [--check]
araihu-assets verify
araihu-assets catalog
araihu-assets export --output <directory>
```

`vendor` is the only networked command. All other commands build from the
manifest, promoted brand masters, and pinned local UI inputs. `catalog`
strictly validates the published catalog before reporting it.

`export` writes only release files below the selected output directory. It
rejects traversal, symlinks, invalid paths, and different-byte collisions;
existing identical files are idempotent. Treat a collision error as a request
to choose a separate output directory or reconcile the consumer's copy.

## Catalog and sprites

After Task 10 publishes generated output, `dist/catalog.json` is schema v1,
language-neutral metadata for generated files only. It records canonical name,
namespace, variant dimensions,
`spriteSymbol`, color behavior, license, source label, and SHA-256. Public
paths do not contain `v11`; `identityRevision: 11` remains metadata.

Brand marks live in namespace `brand`; Heroicons interface icons live in
namespace `ui` under source `heroicons`, alias `hi`, release `v2.2.0`. Resolve
names and paths from the catalog. For SVG entries with a nonempty
`spriteSymbol`, use the declared symbol from the corresponding generated sprite:

- `dist/icons/brand/sprite.svg`
- `dist/icons/ui/sprite.svg`

Only catalog entries marked `monochrome` or `tintable` may use `currentColor`.
Protected brand artwork retains its designed colors. No client-language source
generation is provided by this CLI; consumers own any project-local bindings.

## Platform files and archives

The Task 10 release build emits individual brand/UI SVGs, web/Android/Apple
platform packages, catalog, checksums, notices, licenses, and deterministic
`.tar.gz` and `.zip` release archives under `dist/releases/`. Platform launchers
have their own safe-area, raster, and metadata contracts; do not substitute a
general SVG for a launcher asset.

Released archives and `catalog.json` are the interoperability boundary. They
remain usable without Go once Task 10 has published them.

## Licensing

Repository code and documentation are Apache-2.0 unless a more specific notice
applies. Arai Hû names, logos, and marks are brand assets: preserve notices and
attribution, do not imply endorsement or affiliation, and obtain permission
for modification, standalone redistribution, merchandise, or another identity.
See [NOTICE](NOTICE).

Heroicons are third-party interface icons, licensed under upstream MIT terms;
their released notice is `dist/licenses/heroicons-MIT.txt`. Do not treat the
repository Apache license or Arai Hû brand terms as relicensing Heroicons.

## History and integration

The release tree retains one current source of truth. Earlier concept trees,
reviews, screenshots, and exported PDFs live in Git history; see
[identity evolution](docs/history/identity-evolution.md). Consumer integration,
including Goshtoso's generic sprite boundary and catalog-first local binding
generation, is documented in [docs/integration](docs/integration/).

The temporary V11 calibration scaffold remains only for the pre-P1
`proof-check` gate. P1 replaces it with generated `dist/proof` output after the
release-candidate catalog is available; it must not become a public consumer
path or a second source of truth.
