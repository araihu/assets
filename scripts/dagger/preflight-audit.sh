#!/usr/bin/env bash
set -euo pipefail

audit_image='node:22.14.0-bookworm-slim@sha256:1c18d9ab3af4585870b92e4dbc5cac5a0dc77dd13df1a5905cea89fc720eb05b'

# Core calls do not load this repository's Dagger module or TypeScript runtime.
dagger core --help | grep -F 'container' >/dev/null
dagger core container \
  from --address="$audit_image" \
  with-directory --path=/src/.dagger --source=.dagger \
  with-workdir --path=/src \
  with-exec --args=npm,--prefix,.dagger,audit,--package-lock-only,--omit=dev,--audit-level=high \
  stdout >/dev/null
