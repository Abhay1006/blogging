#!/data/data/com.termux/files/usr/bin/bash
# Continuous deployment for the Termux tablet.
#
# CI builds the arm64 bundle for every commit on main that passes the backend
# and frontend checks, and publishes it as the `tablet-deploy` GitHub release
# (.github/workflows/ci.yml). Nothing is ever built here: the Go toolchain's
# memory use trips Android's low-memory killer. This script runs on the tablet,
# watches that release, and when it changes:
#
#   1. downloads the bundle and verifies its checksum,
#   2. stops the server and snapshots lumina.db,
#   3. swaps in the new binary, frontend and scripts,
#   4. starts the server and waits for /api/health to answer,
#   5. puts the previous version back if it does not.
#
# Usage:
#   ~/lumina/tab-deploy.sh            deploy once if there is a new release
#   ~/lumina/tab-deploy.sh watch [s]  check every s seconds (default 60), forever
#   ~/lumina/tab-deploy.sh restart    restart the server without deploying
#   ~/lumina/tab-deploy.sh status     show the deployed build and what runs
#
# Settings (environment variables, or ~/lumina/.deploy/env; all optional):
#   DEPLOY_URL      where the release assets live
#                   (default https://github.com/Abhay1006/blogging/releases/download/tablet-deploy)
#   HEALTH_TIMEOUT  seconds to wait for the server  (default 60)
#   PORT            port to health-check            (default: PORT in .env, else 3500)
#
# See docs/TAB-DEPLOY.md for the one-time setup.

set -u

DIR="$HOME/lumina"
cd "$DIR" || exit 1

STATE="$DIR/.deploy"
mkdir -p "$STATE" "$DIR/logs"
# shellcheck source=/dev/null
[ -f "$STATE/env" ] && . "$STATE/env"

DEPLOY_URL="${DEPLOY_URL:-https://github.com/Abhay1006/blogging/releases/download/tablet-deploy}"
HEALTH_TIMEOUT="${HEALTH_TIMEOUT:-60}"
METRICS="127.0.0.1:20250" # identifies the blog's tunnel; see start.sh
ASSET="lumina-arm64.tar.gz"

INCOMING="$STATE/incoming"
PREVIOUS="$STATE/previous"
LAST_RELEASE="$STATE/last-release" # release sha last handled, deployed or not
FAILED="$STATE/failed"             # release sha that failed and was rolled back
LOCK="$STATE/lock"
LOG="$DIR/logs/deploy.log"
SERVER="$DIR/lumina-server"

log() {
	printf '%s %s\n' "$(date '+%Y-%m-%d %H:%M:%S')" "$*" | tee -a "$LOG"
}

port() {
	if [ -n "${PORT:-}" ]; then
		echo "$PORT"
		return
	fi
	local p=""
	[ -f .env ] && p="$(sed -n 's/^[[:space:]]*PORT[[:space:]]*=[[:space:]]*//p' .env | tail -n 1 | tr -d "\"'\r ")"
	echo "${p:-3500}"
}

# --- process control --------------------------------------------------------

server_running() { pgrep -f "$SERVER" >/dev/null 2>&1; }

healthy() {
	curl -fsS -o /dev/null --max-time 5 "http://127.0.0.1:$(port)/api/health" 2>/dev/null
}

# Stops only the server. stop.sh would also take down the blog's tunnel, which
# a deploy has no reason to touch.
stop_server() {
	pkill -f "$SERVER" 2>/dev/null || return 0
	local _
	# The server allows itself 10s to drain connections on SIGTERM.
	for _ in $(seq 1 15); do
		server_running || return 0
		sleep 1
	done
	pkill -KILL -f "$SERVER" 2>/dev/null
	sleep 1
}

# start.sh is idempotent: it starts the server and the tunnel only if each is
# not already running.
start_all() {
	"$DIR/start.sh" >>"$LOG" 2>&1
}

wait_healthy() {
	local waited=0
	while [ "$waited" -lt "$HEALTH_TIMEOUT" ]; do
		healthy && return 0
		sleep 2
		waited=$((waited + 2))
	done
	return 1
}

restart_server() {
	log "restarting the server"
	stop_server
	start_all
	if wait_healthy; then
		log "server is up on port $(port)"
		return 0
	fi
	log "ERROR: server did not answer /api/health within ${HEALTH_TIMEOUT}s (see logs/server.log)"
	return 1
}

# Revives the server or the tunnel if Android killed either. Nothing else
# supervises them, so without this they stay dead until someone notices.
ensure_running() {
	[ -x "$SERVER" ] || return 0
	if ! server_running; then
		log "server is not running, starting it"
		start_all
	elif [ -f "$DIR/.cloudflared-token" ] && ! pgrep -f "$METRICS" >/dev/null 2>&1; then
		log "tunnel is not running, starting it"
		start_all
	fi
}

# --- deploy -------------------------------------------------------------------

fetch() {
	curl -fsSL --retry 3 --retry-delay 5 --max-time 600 -o "$2" "$DEPLOY_URL/$1"
}

# Moves the deployed files (binary, frontend, scripts) from directory $1 into
# $DIR. mv replaces each file by rename, so a running binary or this very
# script keeps its old copy until it exits.
install_from() {
	local src="$1" f
	[ -e "$src/lumina-server" ] && mv -f "$src/lumina-server" "$SERVER"
	if [ -d "$src/public" ]; then
		rm -rf "$DIR/public"
		mv "$src/public" "$DIR/public"
	fi
	for f in "$src"/*.sh "$src/VERSION"; do
		[ -e "$f" ] && mv -f "$f" "$DIR/"
	done
}

deploy_once() {
	local want
	if ! fetch VERSION "$STATE/release-version" 2>>"$LOG"; then
		log "could not fetch the release (offline, or CI has not published one yet)"
		ensure_running
		return 0
	fi
	want="$(head -n 1 "$STATE/release-version")"

	# Only a change in the release triggers a deploy. A manual ./deploy.sh in
	# between is left alone until CI publishes something newer.
	if [ -z "$want" ] || [ "$want" = "$(cat "$LAST_RELEASE" 2>/dev/null)" ]; then
		ensure_running
		return 0
	fi

	local short="${want:0:12}"
	log "new build $short: $(sed -n 2p "$STATE/release-version")"

	rm -rf "$INCOMING"
	mkdir -p "$INCOMING/bundle"
	if ! fetch "$ASSET" "$INCOMING/$ASSET" 2>>"$LOG" ||
		! fetch "$ASSET.sha256" "$INCOMING/$ASSET.sha256" 2>>"$LOG" ||
		! (cd "$INCOMING" && sha256sum -c --status "$ASSET.sha256") 2>>"$LOG"; then
		# CI replaces the assets one at a time, so a check can land mid-upload.
		log "download incomplete or checksum mismatch; will retry on the next check"
		rm -rf "$INCOMING"
		ensure_running
		return 0
	fi
	tar xzf "$INCOMING/$ASSET" -C "$INCOMING/bundle" || {
		log "ERROR: could not unpack $ASSET"
		rm -rf "$INCOMING"
		return 1
	}
	if [ "$(head -n 1 "$INCOMING/bundle/VERSION" 2>/dev/null)" != "$want" ]; then
		log "bundle does not match VERSION yet (upload in progress); will retry"
		rm -rf "$INCOMING"
		return 0
	fi
	if [ ! -f "$INCOMING/bundle/lumina-server" ] || [ ! -f "$INCOMING/bundle/public/index.html" ]; then
		log "ERROR: bundle $short is missing lumina-server or public/index.html; skipping it"
		echo "$want" >"$LAST_RELEASE"
		echo "$want" >"$FAILED"
		rm -rf "$INCOMING"
		return 1
	fi

	log "deploying $short"
	stop_server

	# Keep what is running now, for rollback.
	rm -rf "$PREVIOUS"
	mkdir -p "$PREVIOUS"
	[ -e "$SERVER" ] && cp -p "$SERVER" "$PREVIOUS/"
	[ -d "$DIR/public" ] && cp -R "$DIR/public" "$PREVIOUS/"
	cp -p "$DIR"/*.sh "$PREVIOUS/" 2>/dev/null
	[ -f "$DIR/VERSION" ] && cp -p "$DIR/VERSION" "$PREVIOUS/"
	# The server is stopped, so the database files are consistent. This is a
	# safety copy only; rollback never restores it automatically.
	for f in lumina.db lumina.db-wal lumina.db-shm; do
		[ -f "$DIR/$f" ] && cp -p "$DIR/$f" "$PREVIOUS/"
	done

	install_from "$INCOMING/bundle"
	rm -rf "$INCOMING"
	echo "$want" >"$LAST_RELEASE"

	start_all
	if wait_healthy; then
		rm -f "$FAILED"
		log "deployed $short; server is up on port $(port)"
		return 0
	fi

	log "ERROR: $short did not pass the health check, rolling back (see logs/server.log)"
	echo "$want" >"$FAILED"
	stop_server
	if [ -e "$PREVIOUS/lumina-server" ]; then
		install_from "$PREVIOUS"
		start_all
		if wait_healthy; then
			log "rolled back; $short will be skipped until CI publishes a newer build"
		else
			log "ERROR: rollback failed too; the server may be down. Check logs/server.log"
		fi
	else
		log "ERROR: there was no previous version to roll back to"
	fi
	log "a copy of the database from before the deploy is in $PREVIOUS"
	return 1
}

# --- entry points -------------------------------------------------------------

locked() {
	if ! mkdir "$LOCK" 2>/dev/null; then
		local owner
		owner="$(cat "$LOCK/pid" 2>/dev/null)"
		if [ -n "$owner" ] && kill -0 "$owner" 2>/dev/null; then
			log "another deploy is already running"
			return 0
		fi
		# Left behind by a run that was killed.
		rm -rf "$LOCK"
		mkdir "$LOCK" 2>/dev/null || return 0
	fi
	echo $$ >"$LOCK/pid"
	"$@"
	local status=$?
	rm -rf "$LOCK"
	return $status
}

watch() {
	local interval="${1:-60}" self before
	self="$DIR/tab-deploy.sh"
	before="$(cksum <"$self")"
	# Keep Android from suspending the process while the screen is off.
	termux-wake-lock 2>/dev/null
	trap 'rm -rf "$LOCK"; exit 0' INT TERM
	log "watching $DEPLOY_URL every ${interval}s"
	while true; do
		locked deploy_once
		# A deploy can bring a new version of this script; switch to it.
		if [ "$(cksum <"$self")" != "$before" ]; then
			log "tab-deploy.sh was updated, restarting the watcher"
			exec bash "$self" watch "$interval"
		fi
		sleep "$interval"
	done
}

status() {
	echo "deployed:  $(sed -n 1p VERSION 2>/dev/null | cut -c1-12) $(sed -n 2p VERSION 2>/dev/null)"
	[ -f "$FAILED" ] && echo "skipping:  $(cut -c1-12 "$FAILED") (failed its health check)"
	printf 'server:    '
	if healthy; then echo "healthy on port $(port)"; elif server_running; then echo "running, NOT answering"; else echo "stopped"; fi
	printf 'tunnel:    '
	pgrep -f "$METRICS" >/dev/null 2>&1 && echo "running" || echo "stopped"
	printf 'watcher:   '
	pgrep -f "tab-deploy.sh watch" >/dev/null 2>&1 && echo "running" || echo "not running"
}

case "${1:-once}" in
once) locked deploy_once ;;
watch) watch "${2:-60}" ;;
restart) locked restart_server ;;
status) status ;;
*)
	sed -n '/^# Usage/,/^# Settings/p' "$0" | sed '$d; s/^# \{0,1\}//'
	exit 2
	;;
esac
