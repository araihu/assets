#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
browser=${BROWSER_BIN:-/opt/homebrew/bin/chromium}
output=${1:-"$root/review/logo-system-v2-latest.png"}

"$root/scripts/validate-logo-system.sh"
"$browser" \
  --headless \
  --disable-gpu \
  --no-sandbox \
  --allow-file-access-from-files \
  --hide-scrollbars \
  --window-size=1600,2100 \
  --screenshot="$output" \
  "file://$root/review/logo-system-v2.html"

printf '%s\n' "$output"
