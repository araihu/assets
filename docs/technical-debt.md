# Technical debt

Deferred, non-blocking follow-up from the v0.1.1 asset-channel release work.
Items here must not block downstream integration unless later evidence raises
their severity.

## Pre-tag gates

- Run standalone `make verify` and `go vet ./...` immediately before tagging
  v0.1.1. The release gate already covered the full Go suite, deterministic
  build, proof check, and catalog validation.

## Focused test hardening

- Add isolated theme traversal cases, including backslashes and valid `.css`
  suffixes, instead of combining duplicate-ID and traversal failures.
- Reject or explicitly normalize dot segments in exported channel document
  paths; add a disabled-campaign unresolved-reference regression.
- Rename or expand release safety tests so their names match exercised cases.
- Compare `campaigns resolve` stdout byte-for-byte with canonical encoding and
  require exactly one JSON document followed by EOF.
- Add bundle-digest golden vectors plus `default.json` and `current.json`
  mutation cases. Add malformed accepted-state schema and provenance cases.

## Browser runtime hardening

- Preserve safe symbol-level `fill` and `stroke` presentation attributes when
  copying validated sprite symbols.
- Exercise the sprite validator with hostile XML, executable elements, event
  attributes, URL-valued attributes, and malformed `viewBox` values using a
  real XML parser boundary.

## Release consumer fan-out

- `repository_dispatch` confirms GitHub accepted each event but provides no
  downstream completion receipt. Add consumer acknowledgements only after
  operational evidence justifies a durable receipt protocol.
- Manual retry intentionally replays the release to every enrolled fallback
  consumer. Each consumer must deduplicate by immutable release identity;
  consider a validated per-consumer retry filter if duplicate handling becomes
  operationally noisy.

## Review trigger

Revisit this list after v0.1.1 deployment evidence is stable, or sooner if a
consumer exposes a rendering, validation, or compatibility failure tied to an
item above.
