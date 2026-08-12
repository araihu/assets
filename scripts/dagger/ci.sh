#!/usr/bin/env bash
set -euo pipefail

test "$(go version | awk '{print $3}')" = go1.26.5
go tool muamba verify --strict
go tool muamba generate-go --strict --check --dir internal/acquisition --output muamba_gen.go
./scripts/check-ci-workflow_test.sh
./scripts/check-release-workflows_test.sh
env -u HTTPS_PROXY -u HTTP_PROXY make check
env -u HTTPS_PROXY -u HTTP_PROXY make proof-check
go test ./... -count=1
go vet ./...
make verify
make release
diff -ru --exclude=.cache /baseline /src
echo "ci: complete offline contract passed"
