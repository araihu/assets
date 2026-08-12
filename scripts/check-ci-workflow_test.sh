#!/usr/bin/env bash
set -euo pipefail
repo=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
checker="$repo/scripts/check-ci-workflow.sh"

"$checker" "$repo"
"$repo/scripts/materialize-dagger-input_test.sh"

scratch=$(mktemp -d)
trap 'rm -rf -- "$scratch"' EXIT
mutation=0
mutate_and_reject() {
  label=$1 relative=$2 before=$3 after=$4
  mutation=$((mutation + 1))
  fixture="$scratch/mutation-$mutation"
  mkdir -p "$fixture/.github" "$fixture/docs"
  cp -R "$repo/.github/workflows" "$fixture/.github/workflows"
  cp "$repo/.github/actionlint.yaml" "$fixture/.github/actionlint.yaml"
  cp "$repo/docs/dagger-ci.md" "$fixture/docs/dagger-ci.md"
  cp -R "$repo/.dagger" "$fixture/.dagger"
  cp -R "$repo/scripts" "$fixture/scripts"
  cp "$repo/dagger.json" "$repo/go.mod" "$fixture/"
  git -C "$fixture" init --quiet
  git -C "$fixture" add -A
  ruby - "$fixture/$relative" "$before" "$after" <<'RUBY'
path, before, after = ARGV
text = File.read(path)
abort "test fixture marker missing: #{before}" unless text.include?(before)
File.write(path, text.sub(before, after))
RUBY
  if "$checker" "$fixture" >/dev/null 2>&1; then
    echo "CI checker accepted mutation: $label" >&2
    exit 1
  fi
}

pristine="$scratch/pristine"
mkdir -p "$pristine/.github" "$pristine/docs"
cp -R "$repo/.github/workflows" "$pristine/.github/workflows"
cp "$repo/.github/actionlint.yaml" "$pristine/.github/actionlint.yaml"
cp "$repo/docs/dagger-ci.md" "$pristine/docs/dagger-ci.md"
cp -R "$repo/.dagger" "$pristine/.dagger"
cp -R "$repo/scripts" "$pristine/scripts"
cp "$repo/dagger.json" "$repo/go.mod" "$pristine/"
git -C "$pristine" init --quiet
git -C "$pristine" add -A
"$checker" "$pristine" >/dev/null

mutate_and_reject "Core preflight removed" .github/workflows/ci.yml \
  '        run: scripts/dagger/preflight-audit.sh' '        run: true'
mutate_and_reject "Core preflight reordered after project module" .github/workflows/ci.yml \
  $'      - name: Audit locked Dagger runtime dependencies in Core container\n        shell: bash\n        run: scripts/dagger/preflight-audit.sh\n\n      - name: Run complete CI with Dagger\n        shell: bash\n        run: dagger call ci --source=. --payload=.dagger-inputs/ci.json' \
  $'      - name: Run complete CI with Dagger\n        shell: bash\n        run: dagger call ci --source=. --payload=.dagger-inputs/ci.json\n\n      - name: Audit locked Dagger runtime dependencies in Core container\n        shell: bash\n        run: scripts/dagger/preflight-audit.sh'
mutate_and_reject "Core preflight uses project module" scripts/dagger/preflight-audit.sh \
  'dagger core container' 'dagger call ci'
mutate_and_reject "Core preflight image unpinned" scripts/dagger/preflight-audit.sh \
  'node:22.14.0-bookworm-slim@sha256:1c18d9ab3af4585870b92e4dbc5cac5a0dc77dd13df1a5905cea89fc720eb05b' 'node:22.14.0-bookworm-slim'
mutate_and_reject "preflight docs removed" docs/dagger-ci.md \
  'scripts/dagger/preflight-audit.sh' 'scripts/dagger/removed-audit.sh'
mutate_and_reject "audit commented out" scripts/dagger/ci.sh \
  'npm --prefix .dagger audit --package-lock-only --omit=dev --audit-level=high' '# npm --prefix .dagger audit --package-lock-only --omit=dev --audit-level=high'
mutate_and_reject "audit reordered after runtime command" scripts/dagger/ci.sh \
  'npm --prefix .dagger audit --package-lock-only --omit=dev --audit-level=high' $'test -n "$NETWORK_NONCE"\nnpm --prefix .dagger audit --package-lock-only --omit=dev --audit-level=high'

mutate_and_reject "pull request cache promoted to trusted" scripts/materialize-dagger-input.sh \
  'pull_request) cache_namespace=pr' 'pull_request) cache_namespace=trusted'
mutate_and_reject "protected cache demoted to PR" scripts/materialize-dagger-input.sh \
  'push|workflow_dispatch) cache_namespace=trusted' 'push|workflow_dispatch) cache_namespace=pr'
mutate_and_reject "unknown event promoted to trusted" scripts/materialize-dagger-input.sh \
  "*) fail 'unsupported CI event' ;;" "*) cache_namespace=trusted ;;"
mutate_and_reject "Dagger runtime audit removed" scripts/dagger/ci.sh \
  'npm --prefix .dagger audit --package-lock-only --omit=dev --audit-level=high' 'true'
mutate_and_reject "Dagger npm runtime removed" .dagger/src/index.ts \
  'make nodejs npm python3 ruby' 'make python3 ruby'
mutate_and_reject "host npm runtime restored" .github/workflows/ci.yml \
  '      - name: Run complete CI with Dagger' $'      - name: Host npm\n        run: npm --version\n\n      - name: Run complete CI with Dagger'
mutate_and_reject "host node runtime restored" .github/workflows/ci.yml \
  '      - name: Run complete CI with Dagger' $'      - name: Host node\n        run: node --version\n\n      - name: Run complete CI with Dagger'
mutate_and_reject "numeric prerelease leading zero accepted" scripts/materialize-dagger-input.sh \
  '|[1-9][0-9]*|[0-9]*[A-Za-z-][0-9A-Za-z-]*' '|[0-9]+'
mutate_and_reject "provider LF rejection removed" scripts/materialize-dagger-input.sh \
  'must not end with LF' 'provider value accepted'
mutate_and_reject "PR cache namespace hard-coded to trusted" .dagger/src/index.ts \
  'araihu-ci-v1-assets-${cacheNamespace}-go-build-1.26.5' 'araihu-ci-v1-assets-trusted-go-build-1.26.5'
mutate_and_reject "Dagger base image made mutable" .dagger/src/index.ts \
  'golang:1.26.5-trixie@sha256:98988b42f3293b627bf07c884ff17181a59501769cd8c06c7ba901e0ce2c9853' 'golang:1.26.5-trixie'
mutate_and_reject "mutable rustup-init URL restored" scripts/dagger/install-rsvg.sh \
  'rustup/archive/${RUSTUP_VERSION}/${RUSTUP_TARGET}/rustup-init' 'rustup/dist/${RUSTUP_TARGET}/rustup-init'
mutate_and_reject "rustup-init digest changed" scripts/dagger/install-rsvg.sh \
  '20a06e644b0d9bd2fbdbfd52d42540bdde820ea7df86e92e533c073da0cdd43c' '0000000000000000000000000000000000000000000000000000000000000000'
mutate_and_reject "rustup-init digest verification removed" scripts/dagger/install-rsvg.sh \
  'sha256sum --check --strict "$work/rustup-init.sha256"' 'true # rustup-init digest check removed'
mutate_and_reject "Rust toolchain made mutable" scripts/dagger/install-rsvg.sh \
  'RUST_TOOLCHAIN_VERSION=1.92.0' 'RUST_TOOLCHAIN_VERSION=stable'
mutate_and_reject "Rust toolchain moved outside existing cache" scripts/dagger/install-rsvg.sh \
  'export RUSTUP_HOME=/root/.cargo/rustup' 'export RUSTUP_HOME=/root/.rustup'
mutate_and_reject "distro rustup dependency restored" scripts/dagger/install-rsvg.sh \
  'python3-venv ninja-build pkg-config' 'python3-venv ninja-build pkg-config rustup'
mutate_and_reject "Meson wheel digest changed" scripts/dagger/install-rsvg.sh \
  '0ba4a71fbc060c44721c7b674807598c5af9ea51335073cae7a3e9a95b375c89' '0000000000000000000000000000000000000000000000000000000000000000'
mutate_and_reject "Meson wheel digest verification removed" scripts/dagger/install-rsvg.sh \
  'sha256sum --check --strict "$work/meson-wheel.sha256"' 'true # Meson wheel digest check removed'
mutate_and_reject "Meson version changed" scripts/dagger/install-rsvg.sh \
  'MESON_VERSION=1.3.2' 'MESON_VERSION=1.3.0'
mutate_and_reject "Meson wheel filename made noncanonical" scripts/dagger/install-rsvg.sh \
  'MESON_WHEEL=meson-${MESON_VERSION}-py3-none-any.whl' 'MESON_WHEEL=meson.whl'
mutate_and_reject "Meson index access restored" scripts/dagger/install-rsvg.sh \
  'pip" install --no-index --no-deps' 'pip" install --no-deps'
mutate_and_reject "distro Meson dependency restored" scripts/dagger/install-rsvg.sh \
  'python3-venv ninja-build pkg-config' 'meson ninja-build pkg-config'
mutate_and_reject "exact Meson gate removed" scripts/dagger/install-rsvg.sh \
  'test "$(meson --version)" = "$MESON_VERSION"' 'meson --version'
mutate_and_reject "Cairo minimum gate removed" scripts/dagger/install-rsvg.sh \
  'pkg-config --atleast-version=1.18.0 cairo' 'pkg-config --exists cairo'
mutate_and_reject "Cairo minimum weakened" scripts/dagger/install-rsvg.sh \
  'pkg-config --atleast-version=1.18.0 cairo' 'pkg-config --atleast-version=1.16.0 cairo'
mutate_and_reject "Cairo exact version changed" scripts/dagger/install-rsvg.sh \
  'CAIRO_VERSION=1.18.4' 'CAIRO_VERSION=1.18.2'
mutate_and_reject "PR runner routed to generic lane" .github/workflows/ci.yml \
  'hostinger-vps-pr' 'hostinger-vps'
mutate_and_reject "protected runner routed to generic lane" .github/workflows/acquisition.yml \
  'hostinger-vps-trusted' 'hostinger-vps'
mutate_and_reject "provider expression enters Dagger args" .github/workflows/ci.yml \
  'version: "0.21.8"' $'version: "0.21.8"\n          args: ci --trust-domain=${{ github.event_name }}'
mutate_and_reject "self-hosted exact version gate removed" .github/workflows/ci.yml \
  'test "$(dagger version | awk '\''NR == 1 { print $2 }'\'')" = v0.21.8' \
  'dagger version'
mutate_and_reject "remote TypeScript SDK dependency restored" .dagger/package.json \
  '"@dagger.io/dagger": "./sdk"' '"@dagger.io/dagger": "0.21.8"'

sdk_fixture="$scratch/versioned-sdk"
mkdir -p "$sdk_fixture/.github" "$sdk_fixture/.dagger/sdk" "$sdk_fixture/docs"
cp -R "$repo/.github/workflows" "$sdk_fixture/.github/workflows"
cp "$repo/.github/actionlint.yaml" "$sdk_fixture/.github/actionlint.yaml"
cp "$repo/docs/dagger-ci.md" "$sdk_fixture/docs/dagger-ci.md"
cp -R "$repo/.dagger/." "$sdk_fixture/.dagger/"
cp -R "$repo/scripts" "$sdk_fixture/scripts"
cp "$repo/dagger.json" "$repo/go.mod" "$sdk_fixture/"
printf 'export const fake = true\n' > "$sdk_fixture/.dagger/sdk/index.ts"
git -C "$sdk_fixture" init --quiet
git -C "$sdk_fixture" add --force .dagger/sdk/index.ts
if "$checker" "$sdk_fixture" >/dev/null 2>&1; then
  echo 'CI checker accepted a versioned hand-written SDK fixture' >&2
  exit 1
fi

echo "CI workflow checker: provider/runner/cache/CLI/SDK, anti-fixture, and $mutation mutations passed"
