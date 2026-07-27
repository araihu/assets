#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
concept="$root/concepts/v9"

for product in araihu goshtoso manja paje xisnove; do
  for suffix in a mark mark-reverse favicon logo logo-outlined logo-outlined-reverse; do
    asset="$concept/$product-$suffix.svg"
    xmllint --noout "$asset"
    grep -q 'role="img"' "$asset"
    grep -q 'aria-labelledby="title desc"' "$asset"
    grep -q '<title id="title">' "$asset"
    grep -q '<desc id="desc">' "$asset"
    if head -1 "$asset" | grep -Eq '(^|[[:space:]])(width|height)='; then
      printf 'root SVG has fixed dimensions: %s\n' "$asset" >&2
      exit 1
    fi
  done

  grep -q 'viewBox="0 0 128 128"' "$concept/$product-mark.svg"
  grep -q 'viewBox="0 0 128 128"' "$concept/$product-mark-reverse.svg"
  grep -q 'viewBox="0 0 64 64"' "$concept/$product-favicon.svg"
  grep -q 'viewBox="0 0 720 192"' "$concept/$product-logo-outlined.svg"
  grep -q 'viewBox="0 0 720 192"' "$concept/$product-logo-outlined-reverse.svg"

  for compact in "$concept/$product-a.svg" "$concept/$product-mark.svg" "$concept/$product-mark-reverse.svg" "$concept/$product-favicon.svg"; do
    if grep -Eiq '<text|linearGradient|radialGradient|filter|mask|image|href=' "$compact"; then
      printf 'forbidden compact SVG feature: %s\n' "$compact" >&2
      exit 1
    fi
  done
  for outlined in "$concept/$product-logo-outlined.svg" "$concept/$product-logo-outlined-reverse.svg"; do
    if grep -Eiq '<text|linearGradient|radialGradient|filter|mask|image|href=' "$outlined"; then
      printf 'outlined logo is not self-contained: %s\n' "$outlined" >&2
      exit 1
    fi
  done

  colors=$(grep -hEo '#[0-9A-Fa-f]{6}' \
    "$concept/$product-a.svg" \
    "$concept/$product-mark.svg" \
    "$concept/$product-mark-reverse.svg" \
    "$concept/$product-favicon.svg" | sort -u || true)
  for color in $colors; do
    case $(printf '%s' "$color" | tr '[:upper:]' '[:lower:]') in
      '#07111f'|'#31588f'|'#f3f2e9'|'#c7ff4a') ;;
      *) printf 'unapproved color %s for %s\n' "$color" "$product" >&2; exit 1 ;;
    esac
  done
done

duplicate_clouds=$(
  for product in araihu goshtoso manja paje xisnove; do
    xmllint --xpath "string(/*[local-name()='svg']/*[local-name()='path'][1]/@d)" "$concept/$product-a.svg"
    printf '\n'
  done | sort | uniq -d
)
if [ -n "$duplicate_clouds" ]; then
  printf 'V9 repeats an identical outer cloud container: %s\n' "$duplicate_clouds" >&2
  exit 1
fi

v8_master=$(xmllint --xpath "string(/*[local-name()='svg']/*[local-name()='path'][1]/@d)" "$root/concepts/v8/araihu-a.svg")
v9_master=$(xmllint --xpath "string(/*[local-name()='svg']/*[local-name()='path'][1]/@d)" "$concept/araihu-a.svg")
if [ "$v8_master" != "$v9_master" ]; then
  printf 'V9 changed the Arai Hû master cloud instead of deriving from it\n' >&2
  exit 1
fi

printf 'logo system v9 gates passed\n'
