#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
concept="$root/concepts/v10"

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

  cmp "$concept/$product-mark.svg" "$root/logos/$product-mark.svg"
  cmp "$concept/$product-mark-reverse.svg" "$root/logos/$product-mark-reverse.svg"
  cmp "$concept/$product-favicon.svg" "$root/logos/$product-favicon.svg"
  cmp "$concept/$product-logo-outlined.svg" "$root/logos/$product-logo.svg"

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

grep -Eiq 'cloud' "$concept/araihu-a.svg"
for product in goshtoso manja paje xisnove; do
  if grep -Eiq 'cloud' "$concept/$product-a.svg"; then
    printf 'only Arai Hû may use cloud language: %s\n' "$concept/$product-a.svg" >&2
    exit 1
  fi
done

v8_master=$(xmllint --xpath "string(/*[local-name()='svg']/*[local-name()='path'][1]/@d)" "$root/concepts/v8/araihu-a.svg")
v10_master=$(xmllint --xpath "string(/*[local-name()='svg']/*[local-name()='path'][1]/@d)" "$concept/araihu-a.svg")
if [ "$v8_master" != "$v10_master" ]; then
  printf 'V10 changed the accepted Arai Hû master cloud\n' >&2
  exit 1
fi

for product in goshtoso manja paje xisnove; do
  if grep -Fq "$v8_master" "$concept/$product-a.svg"; then
    printf 'product reused the Arai Hû cloud: %s\n' "$concept/$product-a.svg" >&2
    exit 1
  fi
done

(cd "$root" && shasum -a 256 -c brand/canonical-assets-v10.sha256)

printf 'logo system v10 gates passed\n'
