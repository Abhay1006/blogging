#!/usr/bin/env bash
# Build the Termux/Android deploy bundle: the arm64 server binary plus the
# built frontend, packed as one tarball.
#
# Used by CI (.github/workflows/ci.yml), which publishes the bundle for the
# tablet to pull, and by ./deploy.sh for a manual push from the workstation.
#
# Usage: scripts/build-bundle.sh <out-dir>
#
# Writes:
#   <out-dir>/lumina-arm64.tar.gz         lumina-server, public/, device scripts, VERSION
#   <out-dir>/lumina-arm64.tar.gz.sha256  checksum, in `sha256sum -c` format
#   <out-dir>/VERSION                     commit sha, then its subject line
#
# Expects frontend dependencies to be installed (npm ci in frontend/), Go, and
# cmake plus a C compiler for termux-elf-cleaner.
set -euo pipefail

OUT="${1:?usage: scripts/build-bundle.sh <out-dir>}"
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CACHE="${DEPLOY_CACHE:-$REPO/.deploy-cache}"
CLEANER_VERSION="v3.0.1"

mkdir -p "$OUT"
OUT="$(cd "$OUT" && pwd)"
BUNDLE="$(mktemp -d)"
trap 'rm -rf "$BUNDLE"' EXIT

say() { printf '\033[1;34m==>\033[0m %s\n' "$*"; }
die() { printf '\033[1;31mERROR:\033[0m %s\n' "$*" >&2; exit 1; }

# termux-elf-cleaner rewrites two things Android's loader rejects in a
# Go-produced ELF: the TLS segment alignment (Go emits 8, ARM64 Bionic demands
# at least 64) and some unsupported DF_1_* flags. Pinned to a tag so a build
# never runs whatever happens to be on its default branch.
ensure_cleaner() {
	local src="$CACHE/termux-elf-cleaner-$CLEANER_VERSION"
	CLEANER="$src/build/termux-elf-cleaner"
	[[ -x "$CLEANER" ]] && return 0

	say "Building termux-elf-cleaner $CLEANER_VERSION (one time)"
	mkdir -p "$CACHE"
	[[ -d "$src" ]] || git -c advice.detachedHead=false clone --depth 1 -q -b "$CLEANER_VERSION" \
		https://github.com/termux/termux-elf-cleaner.git "$src"
	cmake -S "$src" -B "$src/build" -DCMAKE_BUILD_TYPE=Release >/dev/null
	cmake --build "$src/build" >/dev/null
	[[ -x "$CLEANER" ]] || die "could not build termux-elf-cleaner"
}

# The frontend is served by the Go binary from the same origin, so the API is
# always at /api in production. Set explicitly because CI has no frontend/.env.
say "Building frontend"
(cd "$REPO/frontend" && VITE_API_URL="${VITE_API_URL:-/api}" npm run build >/dev/null)

# -buildmode=pie is required: Android refuses a static ET_EXEC binary
# ("unexpected e_type: 2"). It only accepts position-independent ET_DYN.
say "Cross-compiling server for arm64 (PIE)"
(cd "$REPO/backend" && CGO_ENABLED=0 GOOS=linux GOARCH=arm64 \
	go build -buildmode=pie -trimpath -ldflags="-s -w" -o "$BUNDLE/lumina-server" .)

ensure_cleaner
say "Patching ELF for Bionic"
"$CLEANER" --api-level 24 "$BUNDLE/lumina-server" >/dev/null

cp -R "$REPO/frontend/dist" "$BUNDLE/public"
cp "$REPO/backend/scripts/"*.sh "$BUNDLE/"
chmod +x "$BUNDLE/lumina-server" "$BUNDLE/"*.sh

# VERSION identifies the build on the device. A build from uncommitted changes
# is marked -dirty so it is never mistaken for the commit it started from.
sha="$(git -C "$REPO" rev-parse HEAD)"
git -C "$REPO" diff --quiet HEAD -- || sha="$sha-dirty"
printf '%s\n%s\n' "$sha" "$(git -C "$REPO" log -1 --format=%s)" >"$BUNDLE/VERSION"
cp "$BUNDLE/VERSION" "$OUT/VERSION"

COPYFILE_DISABLE=1 tar czf "$OUT/lumina-arm64.tar.gz" -C "$BUNDLE" .
# macOS has shasum rather than sha256sum; both print the same format.
if command -v sha256sum >/dev/null 2>&1; then sum=(sha256sum); else sum=(shasum -a 256); fi
(cd "$OUT" && "${sum[@]}" lumina-arm64.tar.gz >lumina-arm64.tar.gz.sha256)

say "Built $(du -h "$OUT/lumina-arm64.tar.gz" | cut -f1) bundle for ${sha:0:12}"
