.PHONY: test vendor generate verify check proof proof-check release themes-check campaigns-check

test:
	go test ./... -count=1

vendor:
	go tool muamba sync --strict
	go tool muamba generate-go --strict --dir internal/acquisition --output muamba_gen.go

generate:
	go run ./cmd/araihu-assets build --offline

verify:
	go run ./cmd/araihu-assets verify

check:
	go tool muamba verify --strict
	go tool muamba generate-go --strict --check --dir internal/acquisition --output muamba_gen.go
	$(MAKE) test
	go run ./cmd/araihu-assets build --offline --check

proof:
	go run ./cmd/araihu-assets proof

proof-check:
	go run ./cmd/araihu-assets proof --check

themes-check:
	go run ./cmd/araihu-assets themes validate

campaigns-check:
	go run ./cmd/araihu-assets campaigns validate
	go run ./cmd/araihu-assets campaigns resolve --date 2026-10-31 >/dev/null

# release validates only local release artifacts and generated proof. It never
# creates tags or pushes.
release:
	go tool muamba verify --strict
	go tool muamba generate-go --strict --check --dir internal/acquisition --output muamba_gen.go
	$(MAKE) check
	$(MAKE) proof-check
	go run ./cmd/araihu-assets catalog
