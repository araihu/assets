#!/usr/bin/env bash
set -euo pipefail

test "$(go version | awk '{print $3}')" = go1.26.5
go tool muamba sync --strict --cache-dir "$MUAMBA_CACHE_DIR"
go tool muamba verify --strict --cache-dir "$MUAMBA_CACHE_DIR"
go tool muamba generate-go --strict --check --dir internal/acquisition --output muamba_gen.go
diff -ru --exclude=.cache /baseline /src
echo "acquisition: locked inputs synchronized and generated source unchanged"
