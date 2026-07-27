# Arai Hû identity v2

## Thesis

Build one family of useful signs, not five decorated app tiles. The system joins
Brazilian constructive geometry with the visual language of each product's real
work. Hard shapes, deliberate negative space, and a restrained signal color
replace generic cloud, pulse, and letter-in-a-square motifs.

## Shared grammar

- Midnight `#07111f` is structure: ink, track, page edge, workflow route.
- Cobalt `#31588f` is depth or secondary information.
- Paper `#f3f2e9` is negative space on dark applications.
- Lime `#c7ff4a` is one meaningful event: horizon, active endpoint, approval,
  or healthy signal. Never use it as decoration.
- Marks use a `128 × 128` construction grid and remain recognizable without a
  containing tile.
- Favicons may add a midnight field, but the mark itself must also work on
  paper and midnight backgrounds.
- No gradients, shadows, fine detail, font-dependent SVG text, or cultural
  costume.

## Product signs

### Arai Hû

An anvil storm cloud: stepped mass, long crown, compact rain signal. Meaning
comes directly from *arai hû*, black or dark cloud in Guarani. Inspiration:
Brazilian constructive geometry and weather-map silhouettes.

### Goshtoso

A continuous promenade wave, derived from Rio's black-and-white pavement
language. The wave is treated as a modular interface flow, not a beach logo.

### Manja

An open technical document with a route cut through its pages. The page fold
represents published documentation; the route and indexed endpoint represent
OpenAPI `paths`.

### Pajé

A durable route folded into a `P`, with five checkpoints matching Resolve,
Execute, Approval, Publish, and Finalize. The highlighted fourth checkpoint is
the explicit publication gate.

### Xisnove

A signal-box track diagram: main line, divergence, protected block, and live
signal. It follows Xisnove's own railway dispatch-room design language and its
external monitoring role.

## Reference boundaries

- Geometric language is informed by Brazilian concrete and neoconcrete work;
  no individual artwork is traced or reproduced.
- Goshtoso references the cultural language of Rio's promenade pavement; it
  does not reproduce a protected logo.
- Manja must not reuse or alter the OpenAPI Initiative logo. Product meaning
  comes from document structure and paths instead.
- Pajé avoids generic Indigenous or religious imagery. Its sign represents the
  software workflow only.

## Wordmark provenance

Outlined lockups use Instrument Sans SemiCondensed Bold from
`Instrument/instrument-sans` commit
`7fa22308a3d0c94ee2b3cd537a1196b65db34a3e`. Instrument Sans is licensed under
SIL OFL 1.1; the complete notice is stored in `LICENSES/`.

Generation uses HarfBuzz shaping before conversion to SVG paths, preserving
kerning and the composed `û`/`é` glyphs while removing runtime font dependency:

```sh
python3 -m venv /tmp/araihu-wordmarks
/tmp/araihu-wordmarks/bin/pip install -r scripts/requirements-wordmarks.txt
/tmp/araihu-wordmarks/bin/python scripts/outline-wordmarks.py \
  --font /path/to/InstrumentSansSemiCondensed-Bold.otf \
  --concept-dir concepts/v2
```
