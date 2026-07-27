#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)

for product in araihu goshtoso manja paje xisnove; do
  mark="$root/concepts/v2/$product-mark.svg"
  reverse="$root/concepts/v2/$product-mark-reverse.svg"
  logo="$root/concepts/v2/$product-logo.svg"
  outlined_logo="$root/concepts/v2/$product-logo-outlined.svg"
  favicon="$root/concepts/v2/$product-favicon.svg"
  xmllint --noout "$mark"
  xmllint --noout "$reverse"
  xmllint --noout "$logo"
  xmllint --noout "$outlined_logo"
  xmllint --noout "$favicon"
  grep -q 'viewBox="0 0 128 128"' "$mark"
  grep -q 'viewBox="0 0 720 192"' "$logo"
  grep -q 'viewBox="0 0 720 192"' "$outlined_logo"
  grep -q 'viewBox="0 0 64 64"' "$favicon"
  if grep -Eiq '<text|linearGradient|radialGradient|filter|image|href=' "$outlined_logo"; then
    printf 'outlined logo is not self-contained: %s\n' "$outlined_logo" >&2
    exit 1
  fi
  for compact in "$mark" "$reverse"; do
    if grep -Eiq '<text|linearGradient|radialGradient|filter|image|href=' "$compact"; then
      printf 'forbidden SVG feature: %s\n' "$compact" >&2
      exit 1
    fi
    colors=$(grep -Eo '#[0-9A-Fa-f]{6}' "$compact" | sort -u || true)
    for color in $colors; do
      case $(printf '%s' "$color" | tr '[:upper:]' '[:lower:]') in
        '#07111f'|'#31588f'|'#f3f2e9'|'#c7ff4a') ;;
        *) printf 'unapproved color %s in %s\n' "$color" "$compact" >&2; exit 1 ;;
      esac
    done
  done
done

printf 'logo system SVG gates passed\n'
