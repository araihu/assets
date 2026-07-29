# Product

<!-- impeccable:product-schema 1 -->

## Platform

web

## Users

Arai Hû designers and developers evaluating shared identity assets before using
them in organization sites, web applications, and mobile applications.

## Product Purpose

The asset repository is the canonical source for reusable Arai Hû identity
files. Its v11 review surface must reveal legibility, optical-scale, padding,
theme, and silhouette problems before assets are promoted into products.
Success means reviewers can compare every product and variant at realistic
rendered sizes, identify concrete failures, and regenerate validated SVGs from
their approved sources.

## Positioning

Review happens in plausible product placements at exact pixel sizes, not only
as large logo artwork. The same repository couples visual evidence with
deterministic derivation and validation.

## Operating Context

Reviewers inspect browser chrome, navigation, product cards, mobile app bars,
home-screen icons, tab bars, notifications, and store-scale artwork. They check
light and dark surfaces plus adaptive-background and transparent variants.

## Capabilities and Constraints

- v11 contains five products, each with icon and horizontal logo assets in
  background and transparent modes.
- SVGs remain self-contained, scalable through `viewBox`, accessible, and
  theme-adaptive through semantic `surface`, `ink`, and `signal` roles.
- Promoted masters under `source/brand/original/` remain geometry authority
  unless small-size review produces evidence for a source revision.
- Generated v11 files must stay reproducible and pass repository validators.
- Evaluation spans 16 px favicon use through 512 px mobile/store artwork.

## Brand Commitments

Use Arai Hû paper, midnight, cobalt, and lime signal palette. Preserve distinct
product signs and canonical outlined wordmark paths. Arai Hû is the organization
name; Goshtoso, Manja, Pajé, and X-9/Xisnove are product identities.

## Evidence on Hand

- Approved source exports: `source/brand/original/`
- Temporary V11 calibration scaffold: `source/brand/proof/v11/`
- Theme contract: `themes/araihu.css`
- Temporary V11 proof: `review/logo-system-v11.html`
- Deterministic derivation and validation: `scripts/derive-logo-system-v11.py`
  and `scripts/validate-logo-system-v11.sh`

No customer claims, adoption metrics, or external endorsements are available
and none should be invented.

## Product Principles

- Judge assets at use size, not authoring size.
- Compare like with like across every product.
- Make optical imbalance measurable and visually obvious.
- Keep generated assets deterministic and consumer-safe.
- Record limitations and intended usage beside the evidence.

## Accessibility & Inclusion

Review controls must be keyboard operable, visibly focused, and legible in both
schemes. Asset previews need useful labels; decorative placement copies may use
empty alternative text only when an adjacent label names the asset.
