#!/data/data/com.termux/files/usr/bin/sh
# Start the Lumina blog server, and its Cloudflare tunnel if a token is present.
#
# This device runs several cloudflared processes, including the one serving SSH
# into it. Everything here is matched narrowly -- by binary path, and by a
# metrics port unique to this tunnel -- so starting or stopping the blog can
# never disturb the SSH tunnel that administers the box.
DIR="$HOME/lumina"
METRICS="127.0.0.1:20250" # marker identifying the blog's tunnel process
mkdir -p "$DIR/logs"
cd "$DIR" || exit 1

# Keep Android from suspending us when the screen turns off.
termux-wake-lock 2>/dev/null

if pgrep -f "$DIR/lumina-server" >/dev/null 2>&1; then
	echo "server: already running"
else
	nohup "$DIR/lumina-server" >>"$DIR/logs/server.log" 2>&1 &
	echo "server: started (pid $!)"
fi

if pgrep -f "$METRICS" >/dev/null 2>&1; then
	echo "tunnel: already running"
elif [ -f "$DIR/.cloudflared-token" ]; then
	# Passed via the environment, not argv, so the token stays out of the
	# process list where any app on the device could read it.
	TUNNEL_TOKEN="$(cat "$DIR/.cloudflared-token")" \
		nohup cloudflared --metrics "$METRICS" --no-autoupdate tunnel run \
		>>"$DIR/logs/cloudflared.log" 2>&1 &
	echo "tunnel: started (pid $!)"
else
	echo "tunnel: no token at $DIR/.cloudflared-token, skipping"
fi
