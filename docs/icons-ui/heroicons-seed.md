# Heroicons UI seed

The catalog ships exactly 67 UI icons from Heroicons `v2.2.0`. Source files are
the explicit `src/16/solid/*.svg` downloads in `muamba.yaml`; no glob or mutable
branch expands this set. Muamba retains each upstream SVG and license under
`internal/acquisition/vendor/heroicons/v2.2.0/`, locks each with SHA-384 SRI,
and generates the embedded acquisition API. `manifests/icons-ui.yaml` is only
the typed `assetmeta` overlay for semantic paths and stable references.

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

Normal builds read only embedded, tracked Muamba inputs. Network access occurs
only through the explicit `make vendor` acquisition workflow; tests, UI builds,
verification, proof generation, and release gates remain offline.
