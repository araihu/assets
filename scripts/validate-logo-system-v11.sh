#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
target="$root/source/brand/proof/v11"

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
      grep -q 'aria-label="' "$asset"
      grep -q '<title>' "$asset"
      grep -q '<desc>' "$asset"
      if grep -Eq '(^|[[:space:]])id="' "$asset"; then
        printf 'inline-unsafe static id remains: %s\n' "$asset" >&2
        exit 1
      fi
      grep -q 'class="araihu-brand-v11"' "$asset"
      grep -q -- '--araihu-logo-surface' "$asset"
      grep -q -- '--araihu-logo-ink' "$asset"
      grep -q -- '--araihu-logo-signal' "$asset"
      grep -q '@media (prefers-color-scheme: dark)' "$asset"
      if head -1 "$asset" | grep -Eq '(^|[[:space:]])(width|height)='; then
        printf 'root SVG has fixed dimensions: %s\n' "$asset" >&2
        exit 1
      fi
      if [ "$mode" = transparent ]; then
        case "$kind" in
          icon) expected_viewbox=$(python3 -c 'import sys,xml.etree.ElementTree as E; print(E.parse(sys.argv[1]).getroot().get("viewBox").split()[2] == E.parse(sys.argv[1]).getroot().get("viewBox").split()[3])' "$asset") ;;
          logo) expected_viewbox=True ;;
        esac
        if [ "$expected_viewbox" != True ]; then
          printf 'transparent icon viewBox is not square: %s\n' "$asset" >&2
          exit 1
        fi
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

python3 - "$target" <<'PY'
import sys
import xml.etree.ElementTree as ET
from pathlib import Path

target = Path(sys.argv[1])
for product in ("araihu", "goshtoso", "manja", "paje", "x9"):
    for kind in ("icon", "logo"):
        background = ET.parse(target / f"{product}-{kind}-background.svg").getroot().get("viewBox")
        transparent = ET.parse(target / f"{product}-{kind}-transparent.svg").getroot().get("viewBox")
        expected = "0 0 1024 1024" if kind == "icon" else "0 0 2048 508"
        if background != expected:
            raise SystemExit(f"background safe canvas changed: {product}-{kind}: {background}")
        if transparent == background:
            raise SystemExit(f"transparent optical canvas was not normalized: {product}-{kind}")
PY

printf 'logo system v11 gates passed\n'
