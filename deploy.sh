#!/usr/bin/env bash
# Build Lumina and deploy it to the Termux/Android server.
#
# The server is a single Go binary that serves both the JSON API (under /api)
# and the built frontend, backed by a local SQLite file. Nothing but the
# Cloudflare tunnel talks to the outside world.
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
CACHE="$REPO/.deploy-cache"
STAGE="$(mktemp -d)"
trap 'rm -rf "$STAGE"' EXIT

SSH_OPTS=(-o ConnectTimeout=30 -o ServerAliveInterval=15 -o ServerAliveCountMax=4)

say() { printf '\033[1;34m==>\033[0m %s\n' "$*"; }
die() { printf '\033[1;31mERROR:\033[0m %s\n' "$*" >&2; exit 1; }

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

# ensure_cleaner builds termux-elf-cleaner, which rewrites two things Android's
# loader rejects in a Go-produced ELF: the TLS segment alignment (Go emits 8,
# ARM64 Bionic demands at least 64) and some unsupported DF_1_* flags.
ensure_cleaner() {
	CLEANER="$CACHE/termux-elf-cleaner/build/termux-elf-cleaner"
	[[ -x "$CLEANER" ]] && return 0

	say "Building termux-elf-cleaner (one time)"
	mkdir -p "$CACHE"
	[[ -d "$CACHE/termux-elf-cleaner" ]] || \
		git clone --depth 1 -q https://github.com/termux/termux-elf-cleaner.git "$CACHE/termux-elf-cleaner"
	cmake -S "$CACHE/termux-elf-cleaner" -B "$CACHE/termux-elf-cleaner/build" \
		-DCMAKE_BUILD_TYPE=Release >/dev/null
	cmake --build "$CACHE/termux-elf-cleaner/build" >/dev/null
	[[ -x "$CLEANER" ]] || die "could not build termux-elf-cleaner"
}

build() {
	say "Building frontend"
	(cd "$REPO/frontend" && npm run build >/dev/null)

	# -buildmode=pie is required: Android refuses a static ET_EXEC binary
	# ("unexpected e_type: 2"). It only accepts position-independent ET_DYN.
	say "Cross-compiling server for arm64 (PIE)"
	(cd "$REPO/backend" && CGO_ENABLED=0 GOOS=linux GOARCH=arm64 \
		go build -buildmode=pie -trimpath -ldflags="-s -w" -o "$STAGE/lumina-server" .)

	ensure_cleaner
	say "Patching ELF for Bionic"
	"$CLEANER" --api-level 24 "$STAGE/lumina-server"

	# Go binaries compress to roughly half their size, which matters on a link
	# this fragile.
	gzip -9 "$STAGE/lumina-server"
	cp -R "$REPO/frontend/dist" "$STAGE/public"
	COPYFILE_DISABLE=1 tar czf "$STAGE/public.tar.gz" -C "$STAGE" public
	rm -rf "$STAGE/public"
	cp "$REPO/backend/scripts/"*.sh "$STAGE/"

	say "Built $(du -h "$STAGE/lumina-server.gz" | cut -f1) binary, $(du -h "$STAGE/public.tar.gz" | cut -f1) frontend"
}

push() {
	retry ssh "${SSH_OPTS[@]}" "$HOST" "mkdir -p ~/$REMOTE_DIR/logs"
	for f in lumina-server.gz public.tar.gz start.sh stop.sh status.sh; do
		say "Uploading $f"
		retry scp "${SSH_OPTS[@]}" -C "$STAGE/$f" "$HOST:~/$REMOTE_DIR/$f"
	done

	say "Unpacking"
	retry ssh "${SSH_OPTS[@]}" "$HOST" "
		set -e
		cd ~/$REMOTE_DIR
		rm -f lumina-server && gzip -df lumina-server.gz
		chmod +x lumina-server *.sh
		rm -rf public && tar xzf public.tar.gz && rm public.tar.gz
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
