#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)

for product in araihu goshtoso manja paje xisnove; do
  for lane in a b; do
    candidate="$root/concepts/v3/$product-$lane.svg"
    xmllint --noout "$candidate"
    grep -q 'viewBox="0 0 128 128"' "$candidate"
    if grep -Eiq '<text|linearGradient|radialGradient|filter|mask|image|href=' "$candidate"; then
      printf 'forbidden SVG feature: %s\n' "$candidate" >&2
      exit 1
    fi
    colors=$(grep -Eo '#[0-9A-Fa-f]{6}' "$candidate" | sort -u || true)
    for color in $colors; do
      case $(printf '%s' "$color" | tr '[:upper:]' '[:lower:]') in
        '#07111f'|'#31588f'|'#f3f2e9'|'#c7ff4a') ;;
        *) printf 'unapproved color %s in %s\n' "$color" "$candidate" >&2; exit 1 ;;
      esac
    done
  done
done

printf 'logo exploration SVG gates passed\n'
