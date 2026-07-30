# Consumer integration

Start from a released archive or a verified `dist/` tree. Validate
`catalog.json` as schema v1, verify the selected artifact SHA-256, and keep the
catalog's relative path under that release root.

For `v0.1.1` and later, also require `catalog.json.release`, `themes.json.release`, and
`release.json.release` to agree. Verify `release.json` inventory hashes before
using a patch candidate. This checkout's `v0.1.2` files are offline RC inputs;
they are not a published tag or public download until integration approval.
`v0.1.1` remains the independently published promoted default.

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
