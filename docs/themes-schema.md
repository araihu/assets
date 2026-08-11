# Themes schema

`manifests/themes.yaml` is strict source metadata for release stylesheets.
Only schema version `1` and token contract `goshtoso-theme-v1` are accepted.
Each theme has a unique lower-kebab `id` and a safe, relative `.css` path.
Unknown fields and multiple YAML documents are rejected.

The deterministic build captures each declared stylesheet once, publishes it at
the declared path, and emits `dist/themes.json`. The catalog is two-space JSON
with a final newline. Its themes are sorted by `id`; every entry includes its
stylesheet path, token contract, and SHA-256 of the captured bytes.

`themes/araihu.css` is the baseline theme. It keeps Goshtoso Modern typography
and corner geometry (`Lato` and `--radius-sm`) while replacing semantic colors
with the Arai Hû organization palette. `themes/araihu-signal-night.css` is an
inactive campaign proof: it applies only when a consumer explicitly sets
`data-theme="araihu-signal-night"`. Its midnight (`#07111f`), storm
(`#10233d`), paper (`#f7f8f3`), and lime-signal (`#c7ff4a`) foreground pairs
meet normal-text contrast requirements; it does not select a default theme.
