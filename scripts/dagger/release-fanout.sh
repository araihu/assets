#!/usr/bin/env bash
set -euo pipefail
test -n "$EFFECT_NONCE"
test "$NETWORK_NONCE" = "$EFFECT_NONCE"
test -s /fanout/repositories
test -s /fanout/release-dispatch.json
test "$(scripts/release-consumer-fanout.rb repositories manifests/release-consumers.yaml)" = "$(cat /fanout/repositories)"
scripts/release-consumer-fanout.rb dispatch manifests/release-consumers.yaml /fanout/release-dispatch.json
