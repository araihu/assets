# Consumer integration

Start from a released archive or a verified `dist/` tree. Validate
`catalog.json` as schema v2, verify the selected artifact SHA-256, and keep the
catalog's relative path under that release root.

Require `catalog.json.release`, `themes.json.release`, and
`release.json.release` to agree. Verify `release.json` inventory hashes before
using a candidate. This checkout's `v0.2.0` files are offline RC inputs;
they are not a published tag or public download until integration approval.
`v0.1.3` remains the latest published release, while `v0.1.1` remains the
independently published promoted default.

Use generated sprite files only with the `spriteSymbol` recorded by the
catalog. Same-origin external sprite references and inline-document rendering
have different browser behavior; consumers choose the mode. Supply an
accessible label for meaningful icons, or mark decorative icons decorative.

For a patch update, treat the catalog as additive: a new allowlisted icon is
compatible only if every existing canonical name and its namespace, semantics,
and sprite symbol remain unchanged. Do not hard-code an internal V11 proof path
or generate bindings from source-tree files.

Export into a clean release-owned directory. The CLI refuses different-byte
collisions and accepts existing identical files, so a collision never silently
overwrites consumer-owned output.

## Verify a published archive

Use a disposable empty directory and verify the archive before reading its
catalog or copying an asset. `SHA256SUMS` authenticates the GitHub Release
asset; the extracted `checksums.txt` authenticates every member inside the
release root. `release.json` binds the catalog and other release inputs.

```sh
tag=v0.2.0
mkdir release-download release-root
gh release download "$tag" --repo araihu/assets \
  --pattern "araihu-assets-${tag}.tar.gz" \
  --pattern SHA256SUMS --dir release-download
grep "  araihu-assets-${tag}.tar.gz$" release-download/SHA256SUMS \
  > release-download/archive.sha256
(cd release-download && sha256sum --check --strict archive.sha256)
tar -xzf "release-download/araihu-assets-${tag}.tar.gz" -C release-root
(cd release-root && sha256sum --check --strict checksums.txt)
```

On macOS, use `shasum -a 256 -c` in place of `sha256sum --check`. On Windows,
use `Get-FileHash -Algorithm SHA256` and compare the archive digest with the
matching `SHA256SUMS` record. The tar and zip files are not byte-equivalent;
their extracted release members are the interoperable boundary.

## Schema v2 migration

Consumers retaining historical releases must decode catalog schema versions 1
and 2. In schema v2, preserve `canonicalName`, `path`, and `spriteSymbol`
exactly and case-sensitively. Never lowercase a field or derive one field from
another. For example, `brand-developer-icons-tRPC` maps to the literal
canonical name and the safe sprite symbol `devicon-trpc`. Regenerate bindings
from the released catalog and test this mapping explicitly.

For a referenced icon, use the declared symbol from the matching generated
sprite; do not build an SVG ID from a filename:

```html
<svg aria-label="tRPC" role="img" viewBox="0 0 24 24">
  <use href="/assets/icons/brand/developer-icons/sprite.svg#devicon-trpc"></use>
</svg>
```

Keep `NOTICE`, both upstream MIT license files, and the generated provenance
files with any consumer-owned output. The catalog and release metadata describe
the asset; they do not grant trademark permission or replace attribution.

## Typed acquisition metadata

Go consumers may import `github.com/araihu/assets/assetmeta` from `v0.1.3` and
adapt generated acquisition records into `assetmeta.Resource` values. The
package validates and indexes those records, strictly loads a schema-1 YAML
overlay into consumer-defined generic metadata types, and resolves stable
`resource/download` references.

Acquisition tools own versions, URLs, paths, integrity values, content hashes,
materialized files, and embedding. Consumers own metadata types, relationship
validation through explicit `assetmeta.ValidateRefs` calls, ordering, rendering,
and generated domain APIs. `assetmeta` does not import Muamba, infer metadata
semantics, or traverse consumer metadata automatically.

This repository demonstrates the boundary directly: `.muamba.yaml`,
`.muamba.lock.yaml`, and
`internal/acquisition/muamba_gen.go` own icon-pack acquisition, while
`manifests/icons-ui.yaml` retains only consumer semantics and stable refs.
