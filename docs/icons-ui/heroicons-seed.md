# Icon pack source boundary

The catalog ships the complete supported SVG surfaces from two immutable
upstream releases:

- Heroicons `v2.2.0`: all 1,288 optimized icons in `16/solid`, `20/solid`,
  `24/outline`, and `24/solid`.
- Developer Icons `v7.0.1`: all 318 individual default, dark, and light SVGs;
  the upstream aggregate `developer-icons.svg` is excluded because this
  repository deterministically generates its own sprite.

`.muamba.yaml` declares the reviewed HTTPS tag archives and bounded glob rules.
`.muamba.lock.yaml` pins each archive plus every resolved source path,
destination path, size, and SHA-384 digest. Muamba materializes those exact
files under `internal/acquisition/vendor/`; normal build and verification paths
perform no network access. `manifests/icons-ui.yaml` contains only semantic pack
metadata and immutable upstream revisions.

## Distributed names

Heroicons source `optimized/<size>/<style>/<name>.svg` becomes
`dist/icons/ui/heroicons/<size>-<style>-<name>.svg`, with sprite symbol
`hi-<size>-<style>-<name>` in `dist/icons/ui/sprite.svg`.

Developer Icons source `icons/<literal-name>.svg` becomes
`dist/icons/brand/developer-icons/<literal-name>.svg`. Canonical names preserve
the literal upstream spelling, including mixed case such as
`brand-developer-icons-tRPC`; the separate safe sprite symbol is
`devicon-trpc`. Its generated sprite is
`dist/icons/brand/developer-icons/sprite.svg`.

All generated catalog entries, individual SVGs, sprites, provenance documents,
licenses, release inventory, checksums, and archive members are deterministic.
The upstream MIT notices are distributed at
`dist/licenses/heroicons-MIT.txt` and
`dist/licenses/developer-icons-MIT.txt`.
