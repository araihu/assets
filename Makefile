.PHONY: test vendor generate verify check proof proof-check release

test:
	go test ./... -count=1

vendor:
	go run ./cmd/araihu-assets vendor

generate:
	go run ./cmd/araihu-assets build --offline

verify:
	go run ./cmd/araihu-assets verify

check: test
	go run ./cmd/araihu-assets build --offline --check

proof:
	go run ./cmd/araihu-assets proof

proof-check:
	go run ./cmd/araihu-assets proof --check

# release validates only local release artifacts and generated proof. It never
# creates tags or pushes.
release: check proof-check
	go run ./cmd/araihu-assets catalog
