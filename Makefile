.PHONY: test check

test:
	go test ./... -count=1

check: test
