#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
exec "$root/scripts/validate-logo-system-v10.sh"
