#!/data/data/com.termux/files/usr/bin/sh
DIR="$HOME/lumina"
METRICS="127.0.0.1:20250"
printf "server:    "; pgrep -f "$DIR/lumina-server" >/dev/null 2>&1 && echo "running" || echo "stopped"
printf "tunnel:    "; pgrep -f "$METRICS"           >/dev/null 2>&1 && echo "running" || echo "stopped"
printf "health:    "; curl -fsS --max-time 5 http://127.0.0.1:3500/api/health 2>/dev/null || echo "unreachable"
echo
printf "frontend:  "; curl -fsS -o /dev/null -w "HTTP %{http_code}" --max-time 5 http://127.0.0.1:3500/ 2>/dev/null || echo "unreachable"
echo
printf "database:  "; ls -lh "$DIR/lumina.db" 2>/dev/null | awk '{print $5}' || echo "missing"
