# Logo system v4 reference research

V4 uses existing open-source SVGs only as visual references. No upstream path,
shape command, or coordinate sequence is copied into final candidates. Every
candidate in `concepts/v4/` was drawn from a blank `128 × 128` field.

## Selected references

The main reference family is Tabler Icons at commit
`fe319f05d943a17efb29c5089e21364d3975843c`, licensed MIT. Its `24 × 24`
grid, 2 px outline, rounded caps, and controlled negative space provide the
craft baseline.

| Product | Reference element | Inspected source | Extracted principle |
| --- | --- | --- | --- |
| Arai Hû | cloud + storm | [Tabler cloud-storm](https://github.com/tabler/tabler-icons/blob/fe319f05d943a17efb29c5089e21364d3975843c/icons/outline/cloud-storm.svg) | Continuous weather contour; lightning remains separate and dominant. |
| Goshtoso | composable modules | [Tabler components](https://github.com/tabler/tabler-icons/blob/fe319f05d943a17efb29c5089e21364d3975843c/icons/outline/components.svg) | Symmetric module rhythm and a useful central void. |
| Manja | API documentation | [Tabler api-book](https://github.com/tabler/tabler-icons/blob/fe319f05d943a17efb29c5089e21364d3975843c/icons/outline/api-book.svg) | Open publication surface with one operational detail. |
| Pajé | routed workflow | [Tabler route](https://github.com/tabler/tabler-icons/blob/fe319f05d943a17efb29c5089e21364d3975843c/icons/outline/route.svg) | One continuous path with visible start and finish. |
| Xisnove | state signal | [Tabler traffic-lights](https://github.com/tabler/tabler-icons/blob/fe319f05d943a17efb29c5089e21364d3975843c/icons/outline/traffic-lights.svg) | Strong capsule silhouette and three readable states. |

The code of each selected reference was inspected in
<https://www.svgviewer.dev/> at 400 px. Screenshots live in the Codex visual
review artifact for this iteration; they are evidence, not redistributed
source assets.

## Original family rules

- `128 × 128` construction field;
- 14 px midnight primary stroke with round caps and joins;
- 10 px cobalt secondary operation;
- one lime live state per mark;
- transparent background, no app tile, gradient, shadow, filter, or mask;
- object recognition before color and before wordmark;
- shared stroke grammar relates the family without repeating the Arai Hû cloud.

## License boundary

Tabler source remains owned by its contributors and is licensed under MIT.
Reference links and the exact commit are retained for auditability. Candidate
SVGs contain original geometry and remain covered by this repository's
Apache-2.0 license.
