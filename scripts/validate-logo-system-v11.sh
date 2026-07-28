#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
target="$root/concepts/v11"

python3 "$root/scripts/derive-logo-system-v11.py" --check

count=$(find "$target" -maxdepth 1 -type f -name '*.svg' | wc -l | tr -d ' ')
if [ "$count" -ne 20 ]; then
  printf 'expected 20 v11 SVGs, found %s\n' "$count" >&2
  exit 1
fi

for product in araihu goshtoso manja paje x9; do
  for kind in icon logo; do
    for mode in background transparent; do
      asset="$target/$product-$kind-$mode.svg"
      xmllint --noout "$asset"
      grep -q 'role="img"' "$asset"
      grep -q 'aria-labelledby="title desc"' "$asset"
      grep -q '<title id="title">' "$asset"
      grep -q '<desc id="desc">' "$asset"
      grep -q 'class="araihu-brand-v11"' "$asset"
      grep -q -- '--araihu-logo-surface' "$asset"
      grep -q -- '--araihu-logo-ink' "$asset"
      grep -q -- '--araihu-logo-signal' "$asset"
      grep -q '@media (prefers-color-scheme: dark)' "$asset"
      if head -1 "$asset" | grep -Eq '(^|[[:space:]])(width|height)='; then
        printf 'root SVG has fixed dimensions: %s\n' "$asset" >&2
        exit 1
      fi
      if grep -Eq '(fill|stroke)="#[0-9A-Fa-f]{6}"' "$asset"; then
        printf 'hard-coded shape color remains: %s\n' "$asset" >&2
        exit 1
      fi
      if grep -Eiq '<text|linearGradient|radialGradient|filter|mask|image|href=' "$asset"; then
        printf 'v11 SVG is not self-contained: %s\n' "$asset" >&2
        exit 1
      fi
    done
  done
done

printf 'logo system v11 gates passed\n'
