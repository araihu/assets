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
