---
name: Arai Hû Asset Calibration Bench
description: Exact-size identity proofs for web and mobile product surfaces
colors:
  midnight: "#07111f"
  storm: "#10233d"
  cobalt: "#173b72"
  paper: "#f3f2e9"
  mist: "#e4e8eb"
  lime-signal: "#c7ff4a"
  steel: "#718397"
typography:
  display:
    fontFamily: "Instrument Sans, ui-sans-serif, system-ui, sans-serif"
    fontSize: "clamp(2.5rem, 7vw, 5.5rem)"
    fontWeight: 760
    lineHeight: 0.92
    letterSpacing: "-0.035em"
  body:
    fontFamily: "Instrument Sans, ui-sans-serif, system-ui, sans-serif"
    fontSize: "1rem"
    fontWeight: 450
    lineHeight: 1.5
  label:
    fontFamily: "ui-monospace, SFMono-Regular, Menlo, monospace"
    fontSize: "0.75rem"
    fontWeight: 650
    lineHeight: 1.2
    letterSpacing: "0.02em"
rounded:
  control: "8px"
  surface: "14px"
spacing:
  hairline: "1px"
  unit: "8px"
  group: "16px"
  section: "48px"
components:
  control:
    backgroundColor: "{colors.paper}"
    textColor: "{colors.midnight}"
    rounded: "{rounded.control}"
    padding: "8px 12px"
  dark-proof:
    backgroundColor: "{colors.midnight}"
    textColor: "{colors.paper}"
    rounded: "{rounded.surface}"
    padding: "16px"
---

# Design System: Arai Hû Asset Calibration Bench

## Overview

**Creative North Star: "The Calibration Bench"**

Surface feels like precise device-proofing equipment: exact pixel readouts,
repeatable specimens, and recognizable application frames. Brand artwork owns
attention; interface recedes into measured paper and midnight fields. Lime marks
active state or important evidence, never decoration.

**Key Characteristics:** exact-size specimens; flat tonal layers; visible
measurement; dense comparison followed by contextual proof; scheme parity.

## Colors

Paper and midnight form paired proof surfaces. Storm and mist separate working
areas. Cobalt supports navigation; lime is reserved for selection and signals.

**The Signal Ration Rule.** Lime indicates current state or a review finding;
never use it as ambient ornament.

## Typography

Instrument Sans/system UI carries headings and prose. Monospace appears only for
pixel sizes, filenames, ratios, and measurements. Display copy stays below 5.5rem
and body copy below 72ch.

## Layout

Use a wide proofing rail with a sticky control header. Comparison strips scroll
horizontally on narrow screens instead of shrinking exact-size specimens.
Context scenes form an asymmetric responsive grid and collapse to one column
below 760px. Spacing follows an 8px base rhythm.

## Elevation & Depth

Flat by default. Tonal changes and hairlines define work surfaces. Use one low,
offset shadow only where a mobile device or floating browser object must read as
a physical preview.

## Shapes

Controls use 8px corners; large context surfaces use 14px. Device simulations
may use platform-like radii. Previewed assets must never gain arbitrary rounding
unless the placement is explicitly testing an OS mask.

## Components

### Proof rails

Keep specimens at literal CSS pixel sizes, label every stop, and provide a
stable alignment baseline. Never stretch an icon or logo to fill its cell.

### Context scenes

Each scene names the real placement and size. Reproduce enough browser or mobile
chrome to make the judgment concrete, without turning the page into mock-product
theater.

### Controls

Segmented controls expose product, scheme, and asset mode. Selected state uses
midnight on paper or lime on midnight. Every control has a visible focus ring.

## Do's and Don'ts

### Do:

- **Do** show transparent assets on both solid and checker surfaces.
- **Do** keep size labels adjacent to the exact specimen.
- **Do** preserve light/dark and product parity throughout the matrix.
- **Do** state evidence and recommendations separately.

### Don't:

- **Don't** judge only from 100% container-width artwork.
- **Don't** add cosmetic backgrounds to transparent assets.
- **Don't** hide failed small-size specimens or compensate with CSS scaling.
- **Don't** alter approved geometry without a reproducible before/after proof.
