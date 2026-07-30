# Campaigns schema

`manifests/campaigns.yaml` is the strict schema-v1 campaign calendar. It
contains date-bounded presentation records with a theme, toggle icon assets,
and brand replacement assets.

`starts_on` and `ends_on` are UTC calendar dates in exact `YYYY-MM-DD` form.
Both bounds are inclusive. Time-of-day and timezone values are rejected.

Every campaign, including a disabled campaign, must have a unique lower-kebab
ID, a non-inverted date range, a lower-kebab theme and catalog asset names, and
toggle modes of either `asset` or `sprite`. Unknown fields and multiple YAML
documents are rejected. Enabled campaigns cannot overlap; disabled campaigns do
not participate in overlap detection but remain fully validated.

This source-level schema only checks canonical-name form. Channel resolution
validates every campaign, including disabled records, against the selected
immutable catalog and theme catalog. It resolves exact catalog sprite symbols,
brand logo and icon membership, and stylesheet and asset URLs below
`/assets/releases/<release>/` on the configured HTTPS public origin. The
resolved document is canonical two-space JSON with a final newline; its digest
is SHA-256 over the same document with an empty `digest` field.
