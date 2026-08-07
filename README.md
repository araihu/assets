# Arai Hû Assets

Deterministic identity and interface assets for Arai Hû products.
`dist/catalog.json` is the versioned, language-neutral consumer contract for
the `v0.2.0` release candidate.

## Current checkout

Requires Go `1.26.5`. A supported older local Go installation may select that
toolchain with `GOTOOLCHAIN=auto`.

The managed `dist/` tree is the `v0.2.0` release candidate. Its catalog, generated
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

## Install and verify

Install the CLI after the `v0.2.0` tag and GitHub Release are public when you
need to run the repository's offline build and catalog commands:

```sh
# Run only after https://github.com/araihu/assets/releases/tag/v0.2.0 exists.
go install github.com/araihu/assets/cmd/araihu-assets@v0.2.0
```

Until then, use `go run ./cmd/araihu-assets ...` from this candidate checkout.
`araihu-assets verify` is a source-checkout reproducibility command. Run it
from the root of the tagged repository checkout; it reads that checkout's
`.muamba.yaml`, `.muamba.lock.yaml`, acquisition inputs, and `dist/`. An
installed binary by itself does not download or authenticate a GitHub Release
archive. Consumers must verify the published archive and its extracted files
as described below.

### Verify a published release archive

Use a disposable, empty directory for each release verification. `SHA256SUMS`
authenticates the GitHub Release archive; the extracted `checksums.txt` verifies
each listed release file. `release.json` inventories the other managed files,
while `checksums.txt` is intentionally not self-listed. The tar and zip archives
are content-equivalent after extraction, not byte-equivalent archives.

```sh
tag=v0.2.0
mkdir release-download release-root

gh release download "$tag" --repo araihu/assets \
  --pattern "araihu-assets-${tag}.tar.gz" \
  --pattern SHA256SUMS \
  --dir release-download

grep "  araihu-assets-${tag}.tar.gz$" \
  release-download/SHA256SUMS > release-download/archive.sha256
(cd release-download && sha256sum --check --strict archive.sha256)
tar -xzf "release-download/araihu-assets-${tag}.tar.gz" -C release-root
(cd release-root && sha256sum --check --strict checksums.txt)
```

On macOS, use `shasum -a 256 -c` in place of `sha256sum --check`. On Windows,
use `Get-FileHash -Algorithm SHA256` for the downloaded archive and compare
the result with the matching archive record in `SHA256SUMS`. Then verify every
extracted member with PowerShell; the embedded file uses `SHA256  relative/path`
records:

```powershell
$root = (Resolve-Path .\release-root).Path
Get-Content (Join-Path $root checksums.txt) | ForEach-Object {
  $expected, $relative = $_ -split '  ', 2
  $actual = (Get-FileHash (Join-Path $root $relative) -Algorithm SHA256).Hash.ToLowerInvariant()
  if ($actual -ne $expected.ToLowerInvariant()) {
    throw "checksum mismatch: $relative"
  }
}
```

`release.json` binds the catalog and other release inputs. Together, its file
inventory and `checksums.txt` cover every managed extracted file except the
checksum list itself; the outer `SHA256SUMS` record authenticates that checksum
list as part of the downloaded archive.

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
make vendor    # sync SHA-384-locked Muamba inputs and regenerate acquisition APIs
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

The application CLI is offline. Full Heroicons and Developer Icons acquisition
is declared in `.muamba.yaml`; `.muamba.lock.yaml` pins each archive and every
resolved SVG. `make vendor` invokes the exactly pinned Muamba tool. All CLI
commands build from promoted brand masters and SHA-384-verified locked inputs.
`catalog` strictly validates the published catalog before reporting it.

`build`, `verify`, `proof`, `catalog`, and `export` operate on the current
repository checkout. They are not release-download verifiers and do not
replace the archive and extracted-member checks above.

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

The `v0.2.0` `dist/catalog.json` is schema v2, language-neutral metadata for
generated files only. It records canonical name,
namespace, variant dimensions,
`spriteSymbol`, color behavior, license, source label, and SHA-256. Public
paths do not contain `v11`; `identityRevision: 11` remains metadata.

Schema v2 preserves upstream mixed case in canonical names, such as
`brand-developer-icons-tRPC`. `spriteSymbol` remains a separate safe SVG ID;
that entry renders through `devicon-trpc`. Schema v1 remains readable for
immutable historical releases and retains its lower-kebab canonical-name rule.

Brand marks and Developer Icons live in namespace `brand`; Heroicons interface
icons live in namespace `ui` under source `heroicons`, alias `hi`, release
`v2.2.0`. Resolve
names and paths from the catalog. For SVG entries with a nonempty
`spriteSymbol`, use the declared symbol from the corresponding generated sprite:

- `dist/icons/brand/sprite.svg`
- `dist/icons/brand/developer-icons/sprite.svg`
- `dist/icons/ui/sprite.svg`

Only catalog entries marked `monochrome` or `tintable` may use `currentColor`.
Protected brand artwork retains its designed colors. No client-language source
generation is provided by this CLI; consumers own any project-local bindings.

## Platform files and archives

The `v0.2.0` build emits individual brand/UI SVGs, web/Android/Apple platform
packages, catalog, checksums, notices, licenses, the self-contained
`dist/proof/**` review site, and deterministic `.tar.gz` and `.zip` archives
under `dist/releases/`. Platform launchers
have their own safe-area, raster, and metadata contracts; do not substitute a
general SVG for a launcher asset.

The catalog is the current interoperability boundary. Release archives contain
the complete managed distribution, including all `dist/proof/**` local assets;
review screenshots and critique files remain outside release membership.

`v0.2.0` is an offline release candidate in this checkout, not a published tag
or download. Consumers must wait for integration approval before treating its
archive names or module version as publicly available.

`v0.1.1` remains the published promoted default until an explicit promotion
changes `manifests/default.yaml`; `v0.1.3` remains the latest published release
until this candidate is tagged. This minor candidate changes the catalog schema
to preserve literal upstream icon names and adds the complete pinned Heroicons
and Developer Icons surfaces.

## Licensing

Repository code and documentation are Apache-2.0 unless a more specific notice
applies. Arai Hû names, logos, and marks are brand assets: preserve notices and
attribution, do not imply endorsement or affiliation, and obtain permission
for modification, standalone redistribution, merchandise, or another identity.
See [NOTICE](NOTICE).

Heroicons are third-party interface icons, licensed under upstream MIT terms;
their released notice is `dist/licenses/heroicons-MIT.txt`. Do not treat the
repository Apache license or Arai Hû brand terms as relicensing Heroicons.

Developer Icons are third-party brand icons, licensed under upstream MIT terms;
their released notice is `dist/licenses/developer-icons-MIT.txt`. Preserve each
project's own trademarks and brand usage requirements independently of that
code license.

## History and integration

The release tree retains one current source of truth. Earlier concept trees,
reviews, screenshots, and exported PDFs live in Git history; see
[identity evolution](docs/history/identity-evolution.md). Consumer integration,
including Goshtoso's generic sprite boundary and catalog-first local binding
generation, is documented in [docs/integration](docs/integration/).

Those guides also define release-archive verification, schema-v2 migration,
exact `canonicalName` and `spriteSymbol` handling, and the provenance boundary
for consumer-local generators.

The temporary V11 calibration scaffold remains only for historical reference.
Generated `dist/proof` is release evidence, not a public consumer path or a
second source of truth.
