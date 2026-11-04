#!/usr/bin/env bash
# Boot the exact bundle that will be shipped and check it serves the blog.
#
# CI runs this on the arm64 tarball from scripts/build-bundle.sh before
# publishing it, so a bundle that cannot start, serves no frontend, or was
# built against the wrong API URL never reaches the tablet. On an x86 machine
# the arm64 binary runs under qemu.
#
# Usage: scripts/smoke-bundle.sh <lumina-arm64.tar.gz>
#
# Needs curl, and on non-arm64 hosts qemu-user-static plus libc6-arm64-cross
# (Debian/Ubuntu: apt-get install qemu-user-static libc6-arm64-cross).
set -euo pipefail

TARBALL="${1:?usage: scripts/smoke-bundle.sh <lumina-arm64.tar.gz>}"
PORT="${SMOKE_PORT:-3599}"
BASE="http://127.0.0.1:$PORT"

WORK="$(mktemp -d)"
SERVER_PID=""
cleanup() {
	[ -n "$SERVER_PID" ] && kill "$SERVER_PID" 2>/dev/null && wait "$SERVER_PID" 2>/dev/null
	rm -rf "$WORK"
}
trap cleanup EXIT

say() { printf '\033[1;34m==>\033[0m %s\n' "$*"; }
fail() {
	printf '\033[1;31mSMOKE TEST FAILED:\033[0m %s\n' "$*" >&2
	echo "--- server log ---" >&2
	cat "$WORK/server.log" >&2 2>/dev/null || true
	exit 1
}

tar xzf "$TARBALL" -C "$WORK"
[ -x "$WORK/lumina-server" ] || fail "bundle has no executable lumina-server"
[ -f "$WORK/public/index.html" ] || fail "bundle has no public/index.html"

run=(./lumina-server)
if [ "$(uname -m)" != "aarch64" ] && [ "$(uname -m)" != "arm64" ]; then
	command -v qemu-aarch64-static >/dev/null ||
		fail "not on arm64 and qemu-aarch64-static is missing (apt-get install qemu-user-static libc6-arm64-cross)"
	run=(qemu-aarch64-static -L /usr/aarch64-linux-gnu ./lumina-server)
fi

say "Starting the bundled server on port $PORT"
(
	cd "$WORK"
	exec env PORT="$PORT" DB_PATH="$WORK/smoke.db" STATIC_DIR="$WORK/public" \
		JWT_SECRET="$(head -c 32 /dev/urandom | od -An -tx1 | tr -d ' \n')" \
		"${run[@]}"
) >"$WORK/server.log" 2>&1 &
SERVER_PID=$!

for _ in $(seq 1 60); do
	curl -fsS -o /dev/null --max-time 2 "$BASE/api/health" 2>/dev/null && break
	kill -0 "$SERVER_PID" 2>/dev/null || fail "server exited during startup"
	sleep 1
done

check() { # name, path, expected HTTP status
	local code
	code="$(curl -s -o "$WORK/body" -w '%{http_code}' --max-time 10 "$BASE$2")"
	[ "$code" = "$3" ] || fail "$1: GET $2 returned $code, expected $3"
	say "ok: $1"
}

check "API health" /api/health 200
grep -q '"ok"' "$WORK/body" || fail "/api/health did not report ok"
check "API reads the database" /api/blogs 200
check "frontend shell" / 200
script="$(grep -o '/assets/index-[^"]*\.js' "$WORK/body" | head -n 1)"
[ -n "$script" ] || fail "index.html does not reference the app bundle"
check "client-side route falls back to the shell" /login 200
check "frontend bundle" "$script" 200
# The frontend must call the API on the same origin. A build that picked up a
# development .env would point at localhost instead, and every request would
# fail in the browser while all of the checks above still pass.
grep -q 'localhost:10000' "$WORK/body" && fail "frontend bundle calls http://localhost:10000; build with VITE_API_URL=/api"
grep -qE "[\`\"']/api[\`\"']" "$WORK/body" || fail "frontend bundle does not use /api as its API base"
say "ok: frontend calls the API at /api"

say "Smoke test passed"
