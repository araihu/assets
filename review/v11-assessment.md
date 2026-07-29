# V11 size and placement assessment

## Finding

Approved v11 paths were centered inside source-sized canvases, but transparent
icons occupied inconsistent fractions of a nominal square slot:

| Product | Before width | Before height |
| --- | ---: | ---: |
| Arai Hû | 37% | 26% |
| Goshtoso | 41% | 40% |
| Manja | 58% | 48% |
| Pajé | 53% | 45% |
| X-9 / Xisnove | 63% | 66% |

At 16–32 CSS px this made Arai Hû and Goshtoso appear materially smaller than
the other products. The issue was canvas framing, not path geometry.

## Change

`scripts/derive-logo-system-v11.py` now gives transparent icons a square optical
viewBox with their dominant art dimension at 76%. Transparent logos receive 8%
art-relative padding on every side. Background variants keep the approved full
1024×1024 icon and 2048×508 logo canvases.

After normalization, dominant icon occupancy is 76% for every product within
rounding; transparent logo art occupies 86% of both viewBox axes. Derivation
checks still prove every path and geometry attribute matches its Recraft source.

## Placement guidance

- **16 px favicon/browser tab:** transparent icon. Never use a horizontal logo.
- **20–32 px app navigation and tab bars:** transparent icon.
- **24–40 px site/app header:** transparent logo where width permits; otherwise
  use the transparent icon plus a text product name.
- **40–96 px product cards and empty states:** transparent icon or logo on a
  controlled surface.
- **PWA launcher and install surfaces:** use the `any` and `maskable` pairs in
  `dist/v11/web/<product>/` and merge its manifest fragment into the app.
- **Android launchers:** copy `dist/v11/android/<product>/res/`. Adaptive layers,
  API 33 monochrome artwork, the 66/108 safe square, and legacy densities are
  already encoded.
- **iOS/iPadOS and App Store:** add the generated `Assets.xcassets` catalog from
  `dist/v11/apple/<product>/`. Its 1024 px light, dark, and grayscale tinted
  masters are opaque and Xcode-ready.
- **Transparent launcher artwork:** avoid. OS masks and user wallpapers are not
  controlled surfaces.

Canonical background SVGs are review/source assets, not complete native app-icon
packages. Platform exports are generated deterministically by
`scripts/build-platform-assets-v11.py` and checked by
`scripts/validate-platform-assets-v11.sh`.

## Visual gate

Current Chromium proofs keep each sign recognizable at 16, 20, and 24 px after
normalization. Rasterization still varies by operating system and pixel density,
so these three stops remain a human release gate. Review both schemes at 100%
zoom; do not compensate for a weak sign with per-application CSS scaling.

Evidence lives in `logo-system-v11.html` and the matching
`screenshots/logo-system-v11-*.png` proofs.

## Good-practice gate

- SVGs are path-only, self-contained, responsive through `viewBox`, and have no
  fixed root dimensions. Every geometry path uses a semantic color role.
- Inline copies use a root `aria-label` and ID-free `<title>`/`<desc>` elements,
  avoiding duplicate-ID failures when one asset appears more than once.
- External `<img>` examples reserve intrinsic aspect ratio to prevent layout
  shift; decorative copies may use empty alternative text only beside a label.
- Web manifests declare distinct `any` and `maskable` icons. Native exports are
  not relabeled web SVGs: Android receives layers and density fallbacks, while
  Apple receives an opaque asset catalog and appearance variants.
- Automated gates prove deterministic generation, XML/JSON validity, dimensions,
  Apple opacity, adaptive-icon API split, and safe-area construction. Human
  review remains required at 16–24 px because geometry checks cannot prove
  perceptual recognition.
