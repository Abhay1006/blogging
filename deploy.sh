#!/usr/bin/env bash
# Build Lumina and deploy it to the Termux/Android server.
#
# The server is a single Go binary that serves both the JSON API (under /api)
# and the built frontend, backed by a local SQLite file. Nothing but the
# Cloudflare tunnel talks to the outside world.
#
# Normally CI builds and publishes every green commit on main, and
# tab-deploy.sh on the device installs it (docs/TAB-DEPLOY.md). This script is
# the manual path: first-time setup, or shipping a local build directly.
#
# Everything is built here on the workstation, never on the device. An earlier
# revision ran "go build" over SSH on the tablet; the Go toolchain's memory use
# tripped Android's low-memory killer, which killed sshd and cloudflared and
# took the machine off the network mid-deploy.
set -euo pipefail

HOST="${HOST:-ssh.oniichad.com}"
REMOTE_DIR="${REMOTE_DIR:-lumina}"
PORT="${PORT:-3500}" # 3000 and 4000 are already taken on the device
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
STAGE="$(mktemp -d)"
trap 'rm -rf "$STAGE"' EXIT

SSH_OPTS=(-o ConnectTimeout=30 -o ServerAliveInterval=15 -o ServerAliveCountMax=4)

say() { printf '\033[1;34m==>\033[0m %s\n' "$*"; }

# The tunnel to this device drops under sustained load, so a single failed
# attempt is not a meaningful signal.
retry() {
	local n=1
	until "$@"; do
		if (( n >= 5 )); then
			printf '\033[1;31mFAILED after %d attempts:\033[0m %s\n' "$n" "$*" >&2
			return 1
		fi
		printf '   attempt %d failed, retrying...\n' "$n" >&2
		n=$((n + 1))
		sleep $((n * 3))
	done
}

preflight() {
	say "Checking the tunnel"
	local code
	code=$(curl -sS -o /dev/null -w '%{http_code}' --max-time 20 "https://$HOST/" 2>/dev/null || echo 000)
	if [[ "$code" != "200" ]]; then
		cat >&2 <<-EOF

		The device is not reachable (HTTP $code).
		  530 / error 1033 -> cloudflared on the device is not connected
		  502              -> tunnel is up but sshd on the device is down

		Nothing supervises those processes, so if Android killed them they stay
		dead. Restart them from Termux on the device itself, then re-run.
		EOF
		exit 1
	fi
	say "Device reachable"
}

build() {
	"$REPO/scripts/build-bundle.sh" "$STAGE"
}

push() {
	retry ssh "${SSH_OPTS[@]}" "$HOST" "mkdir -p ~/$REMOTE_DIR/logs ~/$REMOTE_DIR/.deploy"
	say "Uploading lumina-arm64.tar.gz"
	retry scp "${SSH_OPTS[@]}" "$STAGE/lumina-arm64.tar.gz" "$HOST:~/$REMOTE_DIR/.deploy/manual.tar.gz"

	# Unpacked beside the live files and moved into place by rename, so the
	# running binary is never overwritten in place.
	say "Unpacking"
	retry ssh "${SSH_OPTS[@]}" "$HOST" "
		set -e
		cd ~/$REMOTE_DIR
		# Already unpacked by an attempt whose connection dropped afterwards.
		[ -f .deploy/manual.tar.gz ] || exit 0
		rm -rf .deploy/manual && mkdir -p .deploy/manual
		tar xzf .deploy/manual.tar.gz -C .deploy/manual
		mv -f .deploy/manual/lumina-server .deploy/manual/VERSION .deploy/manual/*.sh .
		rm -rf public && mv .deploy/manual/public public
		rm -rf .deploy/manual .deploy/manual.tar.gz
	"
}

# seed_env writes configuration only when it is missing, so the JWT secret stays
# stable across deploys and existing sessions keep working.
seed_env() {
	say "Ensuring configuration"
	retry ssh "${SSH_OPTS[@]}" "$HOST" "
		cd ~/$REMOTE_DIR
		if [ -f .env ]; then
			# The port may have changed even when the rest of .env is intact.
			if grep -q '^PORT=' .env; then
				sed -i 's/^PORT=.*/PORT=$PORT/' .env
			else
				echo 'PORT=$PORT' >> .env
			fi
			echo '   .env present; PORT set to $PORT, secret preserved'
		else
			SECRET=\$(openssl rand -hex 32 2>/dev/null || head -c32 /dev/urandom | od -An -tx1 | tr -d ' \n')
			cat > .env <<-CONF
			PORT=$PORT
			DB_PATH=\$HOME/$REMOTE_DIR/lumina.db
			STATIC_DIR=\$HOME/$REMOTE_DIR/public
			JWT_SECRET=\$SECRET
			ADMIN_EMAIL=admin@lumina.com
			ALLOWED_ORIGINS=http://localhost:5173
			CONF
			chmod 600 .env
			echo '   .env created with a fresh JWT secret'
		fi
	"
}

restart() {
	say "Restarting"
	retry ssh "${SSH_OPTS[@]}" "$HOST" "cd ~/$REMOTE_DIR && ./stop.sh; ./start.sh"
	say "Verifying"
	retry ssh "${SSH_OPTS[@]}" "$HOST" "sleep 4; cd ~/$REMOTE_DIR && ./status.sh; echo '--- log ---'; tail -5 logs/server.log"
}

preflight
build
push
seed_env
restart
say "Done."
