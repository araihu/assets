# Oiê distribution validation

Validated on 2026-11-08 with the pinned librsvg 2.62.1 rasterizer:

- `make generate check verify proof-check` passed, including deterministic
  distribution checks, the full Go suite, all product variants and platform
  icon generation.
- `go vet ./...` passed.
- Oiê-specific checks cover preserved flat colors, dark lettering, adaptive
  output, monochrome facial cutouts and Noto license metadata.

Local Dagger CI was attempted with the repository-pinned CLI and engine
v0.21.11. Module loading stopped before CI execution because the remote Docker
host rejected an overlay snapshot mount with `operation not permitted`.
The already-running v0.21.9 engine is below this module's required version.
No engine configuration was changed to bypass that failure.

This checkout prepares v0.2.7. No release tag, channel promotion or public
archive publication is implied by local generation.
