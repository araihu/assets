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
	go run ./cmd/araihu-assets build --offline

proof-check: proof

# release validates only local release artifacts. Legacy V11 proof remains an
# explicit proof-check target until P1 migrates it to the generated proof.
# It never creates tags or pushes.
release: check
	go run ./cmd/araihu-assets catalog
