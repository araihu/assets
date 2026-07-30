# Arai Hû Assets v0.1.2 release-candidate checklist

## Status

`v0.1.2` is generated and locally verified as an offline release candidate.
It has no tag, push, GitHub Release, channel promotion, or deployment.
`manifests/default.yaml` still promotes the real published `v0.1.1` release.

## Patch purpose

- [x] `internal/releaseinfo.Version` is the sole current release identity:
  `v0.1.2`.
- [x] Release fan-out automation from the first post-`v0.1.1` main revision is
  included.
- [x] Catalog asset semantics remain byte-equivalent after omitting only the
  release field: 302 assets, identity revision 11.
- [x] Theme semantics remain equivalent after omitting only the release field.
- [x] Campaign manifest and campaign runtime remain byte-identical to `v0.1.1`.
- [x] No synthetic `v0.1.0` release or snapshot exists.

## Immutable retention and channels

- [x] Managed `dist/` contains only the newest `v0.1.2` snapshot and archives.
- [x] The tag workflow materializes older promoted `v0.1.1` from its immutable
  GitHub Release archive, validates `SHA256SUMS`, rejects unsafe archive
  members, verifies extracted checksums, and stages only channel contracts.
- [x] Local emulation against the real published `v0.1.1` archive produced
  `latest.json` at `v0.1.2` while `default.json` and `current.json` stayed at
  `v0.1.1`; all three were baseline documents with no active campaign.
- [x] Promoted runtime SHA-256 remained
  `a936193b4fed8120e6cb3423f19d3e2ddb0ba32266dc4e5f02a98f5261853709`.

## Generated immutable bundle

- [x] Catalog: schema 1, release `v0.1.2`, identity revision 11, 302 assets.
- [x] Release metadata: schema 1, runtime version 1, 710 inventoried files.
- [x] `araihu-assets-v0.1.2.tar.gz` SHA-256:
  `033d7eb3deb1966a240de7f3c4b8df9e75db01b70ca1da334ffa22f0ad2c56ba`.
- [x] `araihu-assets-v0.1.2.zip` SHA-256:
  `c6376151a840c69e012cbd1cb5a857794e6c1243dc8d2256adc6c69c59f89876`.
- [x] `release.json` SHA-256:
  `fa9d15f0e0ee9a69c32c951d5b8ad7d0ae57830f19d1f038437e0e538ca1c8c5`.

## Verification

- [x] Focused build identity, deterministic archive, campaign publication, and
  release workflow mutation tests passed.
- [x] `make verify` rebuilt offline and matched the managed distribution.
- [x] `dist/checksums.txt` verified every declared member.
- [x] One end release gate, `make release`, passed.
- [x] `git diff --check` passed.

## Stop condition

No tag, push, GitHub Release, promotion, deployment, or consumer dispatch was
performed.
