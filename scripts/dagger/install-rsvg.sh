#!/usr/bin/env bash
set -euo pipefail

RSVG_VERSION=2.62.1
RSVG_SHA256=b41ca84206242fddd826a2bf76348d7cdf52c1050cbfa060b866e81a252145c3
CARGO_C_VERSION=0.10.10
CARGO_C_SOURCE_SHA256=da2101c5bee6c4bc0d62785c7b79d74a22dd566f93f0530b70d82531d4340b80
CARGO_C_LOCK_SHA256=3d9107cb39d4d3c3503eed03fd668f8c24ad94d2a836f7e8c31f782c31b4a548
RUSTUP_VERSION=1.28.2
RUSTUP_TARGET=x86_64-unknown-linux-gnu
RUSTUP_INIT_SHA256=20a06e644b0d9bd2fbdbfd52d42540bdde820ea7df86e92e533c073da0cdd43c
RUST_TOOLCHAIN_VERSION=1.92.0
MESON_VERSION=1.3.2
MESON_WHEEL=meson-${MESON_VERSION}-py3-none-any.whl
MESON_WHEEL_SHA256=0ba4a71fbc060c44721c7b674807598c5af9ea51335073cae7a3e9a95b375c89
CAIRO_VERSION=1.18.4

if [[ -x /opt/librsvg/bin/rsvg-convert ]] && /opt/librsvg/bin/rsvg-convert --version | grep --fixed-strings --quiet "rsvg-convert version ${RSVG_VERSION}"; then
  exit 0
fi

apt-get update
apt-get install --yes --no-install-recommends \
  build-essential libglib2.0-dev libcairo2-dev libpango1.0-dev libssl-dev \
  libgdk-pixbuf-2.0-dev libxml2-dev python3-venv ninja-build pkg-config
rm -rf /var/lib/apt/lists/*
test "$(pkg-config --modversion cairo)" = "$CAIRO_VERSION"
pkg-config --atleast-version=1.18.0 cairo

work=/tmp/rsvg-build
mkdir -p "$work"
curl --proto '=https' --tlsv1.2 --fail --location --show-error --silent --retry 3 \
  --output "$work/$MESON_WHEEL" \
  "https://files.pythonhosted.org/packages/39/7c/ff115bec047c5127567048db40818b83b47fd0d3bfcfd0d87630d44ed66f/${MESON_WHEEL}"
printf '%s  %s\n' "$MESON_WHEEL_SHA256" "$work/$MESON_WHEEL" > "$work/meson-wheel.sha256"
sha256sum --check --strict "$work/meson-wheel.sha256"
python3 -m venv /opt/meson
"/opt/meson/bin/pip" install --no-index --no-deps "$work/$MESON_WHEEL"

curl --proto '=https' --tlsv1.2 --fail --location --show-error --silent --retry 3 \
  --output "$work/rustup-init" \
  "https://static.rust-lang.org/rustup/archive/${RUSTUP_VERSION}/${RUSTUP_TARGET}/rustup-init"
printf '%s  %s\n' "$RUSTUP_INIT_SHA256" "$work/rustup-init" > "$work/rustup-init.sha256"
sha256sum --check --strict "$work/rustup-init.sha256"
chmod 0755 "$work/rustup-init"
export CARGO_HOME=/root/.cargo
export RUSTUP_HOME=/root/.cargo/rustup
export PATH="/opt/meson/bin:/opt/cargo-c/bin:${CARGO_HOME}/bin:$PATH"
test "$(meson --version)" = "$MESON_VERSION"
"$work/rustup-init" --no-modify-path --profile minimal --default-toolchain "$RUST_TOOLCHAIN_VERSION" -y
test "$(rustup --version | awk 'NR == 1 { print $2 }')" = "$RUSTUP_VERSION"
test "$(rustup run "$RUST_TOOLCHAIN_VERSION" rustc --version | awk '{print $2}')" = "$RUST_TOOLCHAIN_VERSION"

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
rustup run "$RUST_TOOLCHAIN_VERSION" cargo install --locked --path "$work/cargo-c-${CARGO_C_VERSION}" --root /opt/cargo-c
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
