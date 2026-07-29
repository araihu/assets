# Task 9 report: consolidate assets for v0.1.0

## Scope and commit

Task 9 only. Commit containing this report: `chore: consolidate assets for
v0.1.0`. No release-candidate `dist/` generation, tag, push, or ledger edit.

## Pre-delete checkpoint evidence

- `make proof-check` passed before cleanup: V11 derivation/validation and V11
  platform validation both passed.
- All 20 `source/brand/original/*.svg` files were byte-equal and SHA-256-equal
  to their same-named `recraft/*.svg` masters.
- The same 20 sources were byte-equal to
  `862fe9738ab01a6945859ac7f44a8d4b68bd84f1:recraft/<name>.svg`.
- The control-plane checkpoint source is `862fe9738ab01a6945859ac7f44a8d4b68bd84f1`;
  its integrated checkpoint is `4955dfdadabc94199f52d2d6df291ad6c645f807`.

## RED / GREEN

- RED: `go test ./internal/build -run TestReleaseTreeHasNoHistoricalAssetTrees
  -count=1` failed while `concepts`, `recraft`, and `logos` existed.
- GREEN: the same regression passes after explicit cleanup. `make proof-check`,
  `go test ./... -count=1`, `go vet ./...`, and `git diff --check` pass.
- Real-tree `make check` still fails only because legacy tracked `dist/` differs
  from the Task 10 generated release tree; it did not mutate `dist/`.
- Isolated proof at `/tmp/araihu-assets-task9-check.EFl1sw/repo` passed
  `make generate`, offline `make check`, and `make verify`.

## Removed paths

Explicit `git rm` targets only:

- `concepts/` (all historical V2-V10 inputs; V11 moved before removal)
- `recraft/`, `logos/`
- eleven historical `brand/` records: canonical V3/V10 hashes, exploration,
  evaluation, iteration, research, and numbered reviews
- `output/pdf/teste-cego-sinais-araihu-v10.pdf`
- numbered/legacy `review/` HTML/SVG pages and all `review/screenshots/`
- obsolete V3-V10 derivation, review/render, scoring, outline, and validation
  scripts; all targets were passed by explicit name to `git rm`

`archive/` had no tracked or filesystem entry.

## Retained paths

- `source/brand/original/` and `manifests/` remain sole current brand inputs.
- `source/brand/proof/v11/` contains the 20 moved V11 calibration SVGs.
- `dist/v11/`, `review/logo-system-v11.html`, `review/v11-assessment.md`, and
  the V11 derivation/platform/validation scripts remain the minimal pre-P1
  proof scaffold.
- Current Go tool/tests, vendor tree, licenses, plans/specs/control docs, and
  generated current UI assets remain.

## Documentation and consumer contract

README now covers Go 1.26.5 installation, Make/CLI operations, catalog and
sprite use, archive/export collision behavior, and separate Arai Hû brand and
Heroicons MIT terms. `docs/history/identity-evolution.md` records commit-based
history; `docs/integration/` documents catalog-first Goshtoso and consumer
integration, including the additive icon patch rule.

## Sequencing concern and P1 handoff

P1 starts after A4 and replaces the temporary V11 proof with generated,
catalog-driven `dist/proof`. Until that replacement, `make proof-check` must
continue validating the retained scaffold. The V11 platform check deliberately
does not compare its frozen `dist/v11/README.md`, avoiding a Task 9 write to
generated `dist/`; P1 restores whole-tree proof/release ownership.

## Design-detector note

The retained V11 proof file has 61 detector findings (existing typography,
color, and radius literal warnings). Task 9 changed asset paths only; no CSS or
layout lines changed. No suppression was persisted without explicit user
confirmation. `.impeccable/design.json` is also reported stale relative to
`DESIGN.md`; refreshing it is outside Task 9.
