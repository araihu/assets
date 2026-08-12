#!/usr/bin/env bash
set -euo pipefail

RSVG_VERSION=2.62.1
RSVG_SHA256=b41ca84206242fddd826a2bf76348d7cdf52c1050cbfa060b866e81a252145c3
CARGO_C_VERSION=0.10.10
CARGO_C_SOURCE_SHA256=da2101c5bee6c4bc0d62785c7b79d74a22dd566f93f0530b70d82531d4340b80
CARGO_C_LOCK_SHA256=3d9107cb39d4d3c3503eed03fd668f8c24ad94d2a836f7e8c31f782c31b4a548

if [[ -x /opt/librsvg/bin/rsvg-convert ]] && /opt/librsvg/bin/rsvg-convert --version | grep --fixed-strings --quiet "rsvg-convert version ${RSVG_VERSION}"; then
  exit 0
fi

apt-get update
apt-get install --yes --no-install-recommends \
  build-essential libglib2.0-dev libcairo2-dev libpango1.0-dev libssl-dev \
  libgdk-pixbuf-2.0-dev libxml2-dev meson ninja-build pkg-config rustup
rm -rf /var/lib/apt/lists/*

rustup toolchain install 1.92.0 --profile minimal
work=/tmp/rsvg-build
mkdir -p "$work"
curl --fail --location --show-error --silent --retry 3 \
  --output "$work/cargo-c.tar.gz" \
  "https://github.com/lu-zero/cargo-c/archive/refs/tags/v${CARGO_C_VERSION}.tar.gz"
curl --fail --location --show-error --silent --retry 3 \
  --output "$work/Cargo.lock" \
  "https://github.com/lu-zero/cargo-c/releases/download/v${CARGO_C_VERSION}/Cargo.lock"
printf '%s  %s\n' "$CARGO_C_SOURCE_SHA256" "$work/cargo-c.tar.gz" > "$work/cargo-c.sha256"
printf '%s  %s\n' "$CARGO_C_LOCK_SHA256" "$work/Cargo.lock" > "$work/cargo-c-lock.sha256"
sha256sum --check --strict "$work/cargo-c.sha256"
sha256sum --check --strict "$work/cargo-c-lock.sha256"
tar --extract --gzip --file "$work/cargo-c.tar.gz" --directory "$work"
install -m 0644 "$work/Cargo.lock" "$work/cargo-c-${CARGO_C_VERSION}/Cargo.lock"
rustup run 1.92.0 cargo install --locked --path "$work/cargo-c-${CARGO_C_VERSION}" --root /opt/cargo-c
export PATH="/opt/cargo-c/bin:/root/.rustup/toolchains/1.92.0-x86_64-unknown-linux-gnu/bin:$PATH"
test "$(cargo-cbuild --version)" = "cargo-c 0.10.10+cargo-0.86.0"

curl --fail --location --show-error --silent --retry 3 \
  --output "$work/librsvg.tar.xz" \
  "https://download.gnome.org/sources/librsvg/2.62/librsvg-${RSVG_VERSION}.tar.xz"
printf '%s  %s\n' "$RSVG_SHA256" "$work/librsvg.tar.xz" > "$work/librsvg.sha256"
sha256sum --check --strict "$work/librsvg.sha256"
tar --extract --file "$work/librsvg.tar.xz" --directory "$work"
meson setup "$work/librsvg-out" "$work/librsvg-${RSVG_VERSION}" \
  --prefix /opt/librsvg --buildtype release \
  -Ddocs=disabled -Dintrospection=disabled -Dtests=false
meson compile -C "$work/librsvg-out"
meson install -C "$work/librsvg-out"
/opt/librsvg/bin/rsvg-convert --version | grep --fixed-strings "rsvg-convert version ${RSVG_VERSION}"
