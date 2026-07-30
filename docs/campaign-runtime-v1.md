# Campaign runtime v1

`/assets/campaign/v1.js` is an optional deferred progressive-enhancement
runtime. Applications render and retain their own baseline. Loading the runtime
enrolls only the page that includes it:

```html
<html data-theme="araihu" data-theme-source="default">
  <head>
    <script
      src="https://araihu.com/assets/campaign/v1.js"
      data-channel="https://araihu.com/assets/releases/current"
      crossorigin="anonymous"
      integrity="sha384-..."
      defer
    ></script>
  </head>
</html>
```

Pin the published SRI value. Any byte change to `campaign/v1.js` requires a
coordinated consumer SRI update.

## DOM contract

The runtime reads and writes only these root attributes:

- `data-theme`
- `data-theme-source="default|preference|campaign|campaign-opt-out"`
- `data-campaign`

It uses only these enrolled hooks:

- `[data-asset-brand="logo"]` for fixed-size logo images
- `[data-asset-brand="icon"]` for icon links
- `[data-campaign-toggle]` for opt-out buttons
- `[data-campaign-toggle-icon]` for the bounded icon container

Applications must set `data-theme-source="preference"` before the deferred
script executes when a saved explicit theme exists. Explicit preference wins
over campaign state. Changing the source to `preference` later restores
campaign-owned brand URLs without overwriting the newly selected theme.
Changing it back to `default` reruns eligibility.

Campaign activation captures the current theme, source, brand URLs, toggle
state, and toggle contents. Theme CSS, brand images, and the active toggle icon
must all load before one synchronous commit changes the enrolled hooks. A
failed channel, CSS, image, or sprite operation removes staging state and leaves
the baseline unchanged.

## Toggle and storage

The active toggle restores the captured baseline and writes exactly:

```text
araihu.assets.campaign.v1.optout.<campaign-id> = "1"
```

Opt-out is campaign-specific. On reload, it keeps the baseline, exposes
`data-theme-source="campaign-opt-out"`, and renders the declared disabled icon.
An expired campaign restores the captured baseline and hides the toggle.

The runtime does not read any other local-storage key.

## Network and SVG policy

The channel URL is resolved against the document base URL. HTTPS is mandatory
except for `http://localhost`, `http://127.0.0.1`, and `http://[::1]`
development channels. Every resolved theme, brand, and icon URL must:

- use the channel protocol and exact origin;
- have no credentials, query, or fragment;
- be below `/assets/releases/<resolved-release>/`;
- contain no encoded dot, slash, or backslash path segments.

Fetches use CORS mode with credentials omitted. Stylesheets and image preloads
use anonymous CORS. This supports same-origin Ahairu pages and explicitly
enrolled Arai Hû subdomains using the canonical asset origin.

Server data is never passed to `innerHTML`. Direct icons become new `img`
elements. Sprite XML is parsed as SVG; only the exact lower-kebab symbol named
by the validated channel may be selected. Its view box and a restricted SVG
element and attribute set are validated, then its children are copied into a
new inline `svg` through DOM APIs. Other symbols and executable SVG content are
not copied.

## Lifecycle events

Events are dispatched on `document`:

- `araihu:campaign:before-apply`
- `araihu:campaign:applied`
- `araihu:campaign:restored`
- `araihu:campaign:error`

Apply and restore event detail contains only `code`, `campaign`, and
`reducedMotion`. `reducedMotion` reflects
`(prefers-reduced-motion: reduce)`. Applications may use
`before-apply`/`applied` as transition hooks, but should define decorative
transitions only under `@media (prefers-reduced-motion: no-preference)`.

Error detail contains only a stable `code`; thrown objects and server error
text are never exposed. Codes include `channel-url`, `channel-fetch`,
`channel-parse`, `channel-schema`, `schema-version`, `runtime-version`,
`asset-url`, `theme-load`, `image-load`, `sprite-fetch`, `sprite-parse`,
`storage-read`, `toggle-failed`, and `refresh-failed`.

The public API is frozen:

```js
window.AraiHuCampaign.version; // 1
await window.AraiHuCampaign.refresh();
```
