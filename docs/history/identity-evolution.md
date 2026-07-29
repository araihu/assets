# Identity evolution

The release tree contains current inputs and generated release artifacts, not a
gallery of prior identities. Git history is the archive for numbered concepts,
old reviews, screenshots, PDFs, and early logo systems.

## Checkpoints

- `862fe9738ab01a6945859ac7f44a8d4b68bd84f1` preserved the V11 calibration
  source checkpoint.
- `4955dfdadabc94199f52d2d6df291ad6c645f807` integrated that checkpoint into
  the assets release base.
- `160b64f6e3c8b5e302edad6cd13e44a07b088228` promoted the 20 approved masters
  to `source/brand/original/` and introduced declarative brand generation.
- `0bf580b` completed the reviewed brand/platform integration; `35b7948`
  integrated the pinned Heroicons seed.

The promoted masters are the only present geometry authority. The retained V11
calibration files are a temporary proof scaffold generated from those masters,
not a competing historical source tree.

## Migration rule

Use the catalog and published artifacts for integrations. Do not restore
`concepts/`, `recraft/`, `logos/`, `archive/`, old review files, or screenshots
as consumer inputs. Recover an earlier decision with `git show <commit>:<path>`
when needed, then promote a deliberate, reviewed source change through the
manifest/build path.

P1 runs after A4. It replaces the temporary V11 proof with generated,
catalog-driven `dist/proof` output. Until then, `make proof-check` remains an
explicit gate and its minimal scaffold remains intentionally retained.
