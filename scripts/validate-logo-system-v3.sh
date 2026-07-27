#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
concept="$root/concepts/v3"

for product in araihu goshtoso manja paje xisnove; do
  for suffix in mark mark-reverse favicon logo logo-outlined; do
    asset="$concept/$product-$suffix.svg"
    xmllint --noout "$asset"
  done
  cmp "$concept/$product-mark.svg" "$root/logos/$product-mark.svg"
  cmp "$concept/$product-mark-reverse.svg" "$root/logos/$product-mark-reverse.svg"
  cmp "$concept/$product-favicon.svg" "$root/logos/$product-favicon.svg"
  cmp "$concept/$product-logo-outlined.svg" "$root/logos/$product-logo.svg"
  grep -q 'viewBox="0 0 128 128"' "$concept/$product-mark.svg"
  grep -q 'viewBox="0 0 128 128"' "$concept/$product-mark-reverse.svg"
  grep -q 'viewBox="0 0 64 64"' "$concept/$product-favicon.svg"
  grep -q 'viewBox="0 0 720 192"' "$concept/$product-logo.svg"
  grep -q 'viewBox="0 0 720 192"' "$concept/$product-logo-outlined.svg"
  for asset in "$concept/$product-mark.svg" "$concept/$product-mark-reverse.svg" "$concept/$product-favicon.svg" "$concept/$product-logo.svg" "$concept/$product-logo-outlined.svg"; do
    if head -1 "$asset" | grep -Eq '(^|[[:space:]])(width|height)='; then
      printf 'root SVG has fixed dimensions: %s\n' "$asset" >&2
      exit 1
    fi
    grep -q 'role="img"' "$asset"
    grep -q 'aria-labelledby="title desc"' "$asset"
    grep -q '<title id="title">' "$asset"
    grep -q '<desc id="desc">' "$asset"
  done

  for compact in "$concept/$product-mark.svg" "$concept/$product-mark-reverse.svg" "$concept/$product-favicon.svg"; do
    if grep -Eiq '<text|linearGradient|radialGradient|filter|mask|image|href=' "$compact"; then
      printf 'forbidden SVG feature: %s\n' "$compact" >&2
      exit 1
    fi
  done
  if grep -Eiq '<text|linearGradient|radialGradient|filter|mask|image|href=' "$concept/$product-logo-outlined.svg"; then
    printf 'outlined logo is not self-contained: %s\n' "$concept/$product-logo-outlined.svg" >&2
    exit 1
  fi

  colors=$(grep -hEo '#[0-9A-Fa-f]{6}' \
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

(cd "$root" && shasum -a 256 -c brand/canonical-assets-v3.sha256)

printf 'logo system v3 SVG gates passed\n'
