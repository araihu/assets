# Heroicons UI seed

`v0.1.0` ships exactly 67 UI icons from Heroicons `v2.2.0`, pinned to commit
`0435d4ca364a608cc75e2f8683d374e55abbae26`. Source files are the explicitly
allowlisted `src/16/solid/*.svg` paths in `manifests/icons-ui.yaml`; no glob or
mutable branch expands this set. Vendored bytes live under
`vendor/icons/ui/heroicons/v2.2.0/`, with per-file SHA-256 locks in the
manifest and `provenance.json` beside them.

## Selection evidence

57 names normalize existing Goshtoso components and demos by semantics:

```text
arrow-down-tray arrow-down arrow-up arrow-uturn-left arrows-up-down bars-3
bell book-open chart-bar check-circle check chevron-down chevron-left
chevron-right clipboard-document-list clipboard clock cloud-arrow-up
cog-6-tooth cube document-duplicate document-text ellipsis-horizontal
exclamation-circle eye-slash eye face-smile funnel heart home identification
inbox information-circle lock-closed magnifying-glass microphone moon
paint-brush paper-clip pause play plus printer queue-list rectangle-group
scissors sparkles squares-2x2 star sun table-cells user-circle user users
window x-circle x-mark
```

10 additional foundational web/mobile actions complete the seed:

```text
arrow-path                 refresh or retry
arrow-top-right-on-square  open external destination
code-bracket               code or developer surface
ellipsis-vertical          compact overflow actions
folder                     browse or open collection
language                   locale selection
link                       link or copy-link action
pencil-square              edit action
shield-check               verified security state
trash                      delete action
```

The 57 mappings are semantic normalization targets. Goshtoso's historical
inline icons are mixed Heroicons and Bootstrap artwork at several dimensions;
this document does not claim their bytes are Heroicons `v2.2.0` bytes.

## Distributed names and licensing

Each `16/solid/<name>.svg` source becomes
`dist/icons/ui/heroicons/16-solid-<name>.svg`, with sprite symbol
`hi-16-solid-<name>`. `dist/icons/ui/sprite.svg` and the catalog data are
sorted deterministically. The unmodified upstream MIT notice is distributed at
`dist/licenses/heroicons-MIT.txt`.

Normal builds read only tracked vendor files. The opt-in live Sync test is the
sole network path; all ordinary tests and UI builds are offline.
