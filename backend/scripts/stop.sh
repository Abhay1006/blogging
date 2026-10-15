#!/data/data/com.termux/files/usr/bin/sh
# Stop the Lumina blog server and its tunnel -- and nothing else.
#
# Deliberately NOT "pkill cloudflared": that would also kill the tunnel serving
# SSH into this device. The blog's tunnel is identified by its metrics port.
DIR="$HOME/lumina"
METRICS="127.0.0.1:20250"

pkill -f "$DIR/lumina-server" && echo "server: stopped" || echo "server: not running"
pkill -f "$METRICS"           && echo "tunnel: stopped" || echo "tunnel: not running"
