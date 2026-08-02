# Arai Hû Assets v0.1.3 release-candidate checklist

## Status

`v0.1.3` is generated as an offline release candidate. It has no tag, push,
GitHub Release, channel promotion, deployment, or consumer update yet.
`v0.1.2` remains the latest published release and `v0.1.1` remains the
promoted default.

## Patch purpose

- [x] Add public `github.com/araihu/assets/assetmeta` package in the existing
  module; no nested module or independent version.
- [x] Keep acquisition, Muamba, and Goshtoso semantics outside `assetmeta`.
- [x] Preserve strict typed overlays, stable references, immutable inventory
  copies, and opaque integrity/hash strings.
- [x] Advance `internal/releaseinfo.Version` to `v0.1.3`.

## Compatibility

- [x] Catalog matches `v0.1.2` after removing only top-level `release`.
- [x] Themes match `v0.1.2` after removing only top-level `release`.
- [x] Campaign manifest and campaign runtime are byte-identical to `v0.1.2`.
- [x] Catalog contains 302 assets with identity revision 11.

## Generated immutable bundle

- [x] Release metadata inventories 710 files.
- [x] `araihu-assets-v0.1.3.tar.gz` SHA-256:
  `478ae49764e8bb507f709d965599019fe65475e21fd62dda13ac47107b502c8c`.
- [x] `araihu-assets-v0.1.3.zip` SHA-256:
  `fc5d997c617514c4b906b14fc452989d0d91d0f5eabda344c950ace1de0eb455`.
- [x] `release.json` SHA-256:
  `945d0bad21e3048839388e78847ef686cb4ea9300098c864df2299f8443324c8`.

## Verification

- [x] `go test ./... -count=1` passed.
- [x] `go test -race ./assetmeta -count=1` passed.
- [x] `go vet ./...` passed.
- [x] `make check`, `make proof-check`, `make verify`, and `make release`
  passed without generated drift.
- [x] `dist/checksums.txt` verified every declared member.
- [x] `git diff --check` passed.

## Stop condition

No tag, GitHub Release, channel promotion, deployment, or consumer update may
occur before final review and merge.
