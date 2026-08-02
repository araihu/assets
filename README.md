# Arai Hû Assets

Deterministic identity and interface assets for Arai Hû products.
`dist/catalog.json` is the versioned, language-neutral consumer contract for
the `v0.1.3` release candidate.

## Current checkout

Requires Go `1.26.5`. A supported older local Go installation may select that
toolchain with `GOTOOLCHAIN=auto`.

The managed `dist/` tree is the `v0.1.3` release candidate. Its catalog, generated
sprites, catalog-driven proof, and deterministic archives are consumer inputs.
Use a disposable checkout when regenerating it:

```sh
make generate
go run ./cmd/araihu-assets catalog
make check
make verify
```

Run `catalog` only after a successful `make generate`, because it validates the
generated `dist/catalog.json`. `make check` rejects drift across the complete
managed tree, including `dist/proof`.

## Install

After release approval, install and verify the tagged module:

```sh
go install github.com/araihu/assets/cmd/araihu-assets@v0.1.3
araihu-assets verify
```

## Typed acquisition overlays

Release `v0.1.3` adds `github.com/araihu/assets/assetmeta` for applications that
keep acquisition data separate from consumer metadata:

```sh
go get github.com/araihu/assets@v0.1.3
```

Adapt generated acquisition records into an inventory, then load metadata into
consumer-owned Go types:

```go
type metadata struct {
	Entry assetmeta.Ref `yaml:"entry"`
}

inventory, err := assetmeta.NewInventory([]assetmeta.Resource{{
	Name:    "example",
	Version: "1.0.0",
	Downloads: []assetmeta.Download{{
		Name:      "runtime-js",
		URL:       "https://cdn.example/runtime.js",
		Path:      "assets/js/runtime.js",
		Integrity: "sha384-base64-value",
		Hash:      "sha384:0123456789abcdef",
	}},
}})
if err != nil {
	return err
}
overlay := strings.NewReader(`schema: 1
metadata:
  entry: example/runtime-js
`)
document, err := assetmeta.Load[metadata, struct{}, struct{}](overlay, inventory)
if err != nil {
	return err
}
if err := assetmeta.ValidateRefs(document.Inventory(), document.Metadata.Entry); err != nil {
	return err
}
```

Muamba or another acquisition tool owns downloads, integrity, paths, hashes,
and embedding. `assetmeta` owns strict typed YAML association and stable
`resource/download` references. Each application owns metadata meaning and
semantic validation.

For release-maintenance work, stable Make targets are:

```sh
make vendor    # fetch only locked manifest-selected UI sources
make generate  # build offline and atomically replace managed dist/
make verify    # rebuild offline and compare with dist/
make check     # test, then reject generated-output drift
make proof     # atomically regenerate dist/, including dist/proof
make proof-check # reject generated proof/output drift without writing
make themes-check # validate source theme manifest and stylesheets offline
make campaigns-check # validate campaign references and resolve a fixed UTC date
```

`make proof` and `make proof-check` use the same local, catalog-driven builder
as the managed distribution. They require no network access and never invoke
the retired V11 scripts.

## CLI

```text
araihu-assets vendor
araihu-assets build --offline [--check]
araihu-assets verify
araihu-assets proof [--check]
araihu-assets catalog
araihu-assets export --output <directory>
araihu-assets themes validate
araihu-assets campaigns validate
araihu-assets campaigns resolve --date YYYY-MM-DD
araihu-assets campaigns publish --date YYYY-MM-DD --output <directory>
```

`vendor` is the only networked command. All other commands build from the
manifest, promoted brand masters, and pinned local UI inputs. `catalog`
strictly validates the published catalog before reporting it.

`export` writes only release files below the selected output directory. It
rejects traversal, symlinks, invalid paths, and different-byte collisions;
existing identical files are idempotent. Treat a collision error as a request
to choose a separate output directory or reconcile the consumer's copy.

`themes validate`, `campaigns validate`, and `campaigns resolve` are offline
and credential-free. `resolve` writes one canonical channel JSON document to
standard output and writes nothing else. `campaigns publish` writes only
`releases/latest.json`, `releases/default.json`, `releases/current.json`, and
`campaign/v1.js` below its output root; it never creates immutable release
history. Existing identical channel bytes are idempotent, while different-byte
collisions fail without overwriting consumer output.

`dist/` is the newest built release snapshot and therefore supplies
`latest.json` plus the stable runtime. `manifests/default.yaml` independently
selects the promoted baseline for `default.json` and `current.json`. When that
promotion names an older release, provide its immutable offline snapshot at
`releases/<semver>/` with `release.json`, `catalog.json`, `themes.json`, and
`campaigns.json`. `release.json` hashes the captured inputs, so a concurrent
mutation is rejected instead of mixing generations. The command never fetches
that snapshot. A new `dist/` release changes only
`latest.json`; it cannot implicitly move `default.json` or `current.json`.

The tagged release workflow obtains an older promoted snapshot from its real
GitHub Release archive, verifies `SHA256SUMS`, safely extracts it, and stages
only those immutable contracts for channel generation. Historical releases do
not remain in managed `dist/`, and no unpublished release is synthesized.

## Catalog and sprites

The `v0.1.3` `dist/catalog.json` is schema v1, language-neutral metadata for
generated files only. It records canonical name,
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

The `v0.1.3` build emits individual brand/UI SVGs, web/Android/Apple platform
packages, catalog, checksums, notices, licenses, the self-contained
`dist/proof/**` review site, and deterministic `.tar.gz` and `.zip` archives
under `dist/releases/`. Platform launchers
have their own safe-area, raster, and metadata contracts; do not substitute a
general SVG for a launcher asset.

The catalog is the current interoperability boundary. Release archives contain
the complete managed distribution, including all `dist/proof/**` local assets;
review screenshots and critique files remain outside release membership.

`v0.1.3` is an offline release candidate in this checkout, not a published tag
or download. Consumers must wait for integration approval before treating its
archive names or module version as publicly available.

`v0.1.1` remains the published promoted default until an explicit promotion
changes `manifests/default.yaml`; `v0.1.2` remains the latest published release
until this candidate is tagged. The patch candidate adds the `assetmeta` Go
package. Catalog asset semantics, themes, campaign calendar, and campaign
runtime remain compatible with `v0.1.2`.

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

The temporary V11 calibration scaffold remains only for historical reference.
Generated `dist/proof` is release evidence, not a public consumer path or a
second source of truth.
