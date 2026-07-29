#!/usr/bin/env bash
# Validates immutable CI pins and the literal checksum-file format without
# contacting a network service. GitHub Actions runs this before bootstrapping.
set -euo pipefail

workflow=.github/workflows/ci.yml

grep -Fx 'go 1.26.0' go.mod
grep -Fx 'toolchain go1.26.5' go.mod
grep -F "go-version: '1.26.5'" "$workflow"
grep -F 'test "$(go version | awk '\''{print $3}'\'')" = go1.26.5' "$workflow"

grep -F 'actions/setup-go@44694675825211faa026b3c33043df3e48a5fa00 # v6.0.0' "$workflow"
grep -F 'CARGO_C_VERSION: 0.10.10' "$workflow"
grep -F 'CARGO_C_SOURCE_SHA256: da2101c5bee6c4bc0d62785c7b79d74a22dd566f93f0530b70d82531d4340b80' "$workflow"
grep -F 'CARGO_C_LOCK_SHA256: 3d9107cb39d4d3c3503eed03fd668f8c24ad94d2a836f7e8c31f782c31b4a548' "$workflow"
grep -F 'rustup toolchain install 1.92.0 --profile minimal' "$workflow"
grep -F 'rustup run 1.92.0 rustc --version | grep --fixed-strings "rustc 1.92.0"' "$workflow"
grep -F 'rustup run 1.92.0 cargo install --locked --path "$CARGO_C_SOURCE" --root "$CARGO_C_ROOT"' "$workflow"
grep -F 'export PATH="$CARGO_C_ROOT/bin:$HOME/.rustup/toolchains/1.92.0-x86_64-unknown-linux-gnu/bin:$PATH"' "$workflow"
grep -F 'test "$(command -v cargo-cbuild)" = "$CARGO_C_ROOT/bin/cargo-cbuild"' "$workflow"
if grep -F 'cargo +1.92.0 install' "$workflow"; then
  echo 'CI must use rustup run before direct toolchain PATH export' >&2
  exit 1
fi
install_line=$(grep -nF 'rustup run 1.92.0 cargo install' "$workflow" | cut -d: -f1)
path_line=$(grep -nF 'export PATH="$CARGO_C_ROOT/bin:$HOME/.rustup/toolchains/1.92.0-x86_64-unknown-linux-gnu/bin:$PATH"' "$workflow" | cut -d: -f1)
test "$install_line" -lt "$path_line"
grep -F "printf '%s  %s\\n' \"\$RSVG_SHA256\" \"\$RSVG_ARCHIVE\" > \"\$RSVG_CHECKSUM\"" "$workflow"

probe_dir=$(mktemp -d)
artifact="$probe_dir/artifact"
checksum="$probe_dir/artifact.sha256"
printf 'ci checksum probe\n' > "$artifact"
sha256=$(sha256sum "$artifact" | awk '{print $1}')
printf '%s  %s\n' "$sha256" "$artifact" > "$checksum"
test "$(tail -c 1 "$checksum" | od -An -t x1 | tr -d ' ')" = 0a
sha256sum --check --strict "$checksum"
