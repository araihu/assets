#!/usr/bin/env bash
set -euo pipefail

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
repo_dir=$(CDPATH= cd -- "$script_dir/.." && pwd)
output_path=${1:-"$repo_dir/output/pdf/teste-cego-sinais-araihu-v10.pdf"}
browser_bin=${BROWSER_BIN:-}

if [[ -z "$browser_bin" ]]; then
  for candidate in \
    "/opt/homebrew/bin/chromium" \
    "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome" \
    "/Applications/Chromium.app/Contents/MacOS/Chromium"; do
    if [[ -x "$candidate" ]]; then
      browser_bin=$candidate
      break
    fi
  done
fi

if [[ -z "$browser_bin" ]]; then
  printf 'error: set BROWSER_BIN to a Chromium-compatible browser\n' >&2
  exit 1
fi

mkdir -p "$(dirname -- "$output_path")"
"$browser_bin" \
  --headless=new \
  --disable-gpu \
  --allow-file-access-from-files \
  --no-pdf-header-footer \
  --print-to-pdf="$output_path" \
  "file://$repo_dir/review/blind-review-print.html"

printf '%s\n' "$output_path"
