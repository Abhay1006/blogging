# Deploying to the tablet (CI/CD)

Lumina is self-hosted on a Termux/Android tablet. Code is pushed from the PC,
and the tablet installs each change on its own once CI has checked and built
it:

```
PC                  GitHub Actions                                 Tablet
git push ──► main ──► CI: gofmt, vet, Go tests, lint, vitest,
                      frontend build, shellcheck
                         │ all green
                         ▼
                      build arm64 bundle ──► `tablet-deploy` release ◄── tab-deploy.sh watch
                                                                         (checks every 60s)
                                                                         download → verify
                                                                         → swap → restart
                                                                         → health check
                                                                         → roll back if it fails
```

- **CI** (`.github/workflows/ci.yml`) runs on every push and pull request.
  When a push to `main` passes every check, the `publish-tablet` job builds
  the arm64 bundle with `scripts/build-bundle.sh` and uploads it to the
  rolling `tablet-deploy` release (a prerelease, so it never shows as
  "Latest"). A commit that fails CI is never published.
- **CD** (`~/lumina/tab-deploy.sh`, from `backend/scripts/`) runs on the
  tablet and watches that release. The tablet pulls from GitHub, so it needs
  no inbound SSH, webhook or open port, and does not depend on the SSH tunnel
  staying up during a deploy.

This differs from a pull-and-build setup on purpose: **nothing is built on the
tablet.** The Go toolchain's memory use trips Android's low-memory killer,
which takes `sshd` and `cloudflared` down with it. CI cross-compiles the PIE
binary and patches it with termux-elf-cleaner exactly as `deploy.sh` always
has (see the README's "Why the build is the shape it is").

## What a deploy does

1. Downloads `VERSION` from the release. If it names a build the tablet has
   already handled, nothing happens.
2. Downloads the bundle and its `.sha256`, verifies the checksum, and checks
   the bundle's own `VERSION` matches. If CI is still uploading, it retries on
   the next check.
3. Stops the server (not the tunnel), and copies the current binary, frontend,
   scripts and `lumina.db` into `.deploy/previous/`.
4. Moves the new `lumina-server`, `public/` and scripts into `~/lumina/`.
5. Starts the server and waits up to 60 seconds for `/api/health`.
6. If that fails, puts the previous binary, frontend and scripts back, starts
   them, and skips the bad build until CI publishes a newer one. The database
   is **not** restored automatically; the copy from step 3 stays in
   `.deploy/previous/` if you need it.

`.env`, `lumina.db`, `logs/` and `.cloudflared-token` are never touched.

On every check it also restarts the server or the blog's tunnel if Android
killed either, which nothing did before.

## One-time tablet setup

1. **Make sure `~/lumina` exists and has a `.env`.** If you have deployed with
   `./deploy.sh` before, it does. Otherwise run it once from the PC; it
   creates `.env` with a fresh `JWT_SECRET`:

   ```bash
   ./deploy.sh
   ```

   That also copies `tab-deploy.sh` to the tablet. (Without `deploy.sh`, create
   `~/lumina/.env` by hand as in the README, and fetch the script from the
   release: `mkdir -p ~/lumina && cd ~/lumina && curl -fsSL https://github.com/Abhay1006/blogging/releases/download/tablet-deploy/lumina-arm64.tar.gz | tar xz ./tab-deploy.sh`.)

2. **Deploy once by hand, in Termux on the tablet**, to check it works:

   ```bash
   ~/lumina/tab-deploy.sh
   ~/lumina/tab-deploy.sh status
   ```

3. **Keep watching for new builds**:

   ```bash
   ~/lumina/tab-deploy.sh watch      # checks every 60 seconds
   ```

   `watch` takes a wake lock so Android does not suspend Termux with the
   screen off. Also set Android's battery setting for Termux to Unrestricted
   (Apps → Termux → Battery). Run it inside `tmux` if you start it over SSH,
   so it survives the session ending.

4. **Start it when the tablet boots** (optional, recommended). Install
   [Termux:Boot](https://f-droid.org/packages/com.termux.boot/), open it once,
   then:

   ```bash
   mkdir -p ~/.termux/boot
   cat > ~/.termux/boot/lumina <<'EOF'
   #!/data/data/com.termux/files/usr/bin/sh
   termux-wake-lock
   ~/lumina/tab-deploy.sh watch >/dev/null 2>&1 &
   EOF
   chmod +x ~/.termux/boot/lumina
   ```

   `watch` starts the server and tunnel on its first check, so nothing else
   is needed, and it brings them back if they die.

## Day to day

```bash
git push origin main
```

Within a few minutes (CI time plus up to 60 seconds) the tablet is running it.
Follow CI in the repo's **Actions** tab; the run summary shows the published
build. On the tablet: `tail -f ~/lumina/logs/deploy.log`.

## Commands

| Command                                     | What it does                                                    |
| ------------------------------------------- | --------------------------------------------------------------- |
| `~/lumina/tab-deploy.sh`                    | Deploy once if the release changed; revive the server if needed |
| `~/lumina/tab-deploy.sh watch [seconds]`    | Do that forever, every 60 seconds by default                    |
| `~/lumina/tab-deploy.sh status`             | Deployed build, skipped build, server, tunnel and watcher state |
| `~/lumina/tab-deploy.sh restart`            | Restart the server without deploying (e.g. after editing .env)  |
| `~/lumina/stop.sh` / `start.sh` / `status.sh` | Unchanged                                                     |

Optional settings, as environment variables or in `~/lumina/.deploy/env`
(sourced by the script): `DEPLOY_URL` (where the release assets are),
`HEALTH_TIMEOUT` (default `60` seconds), `PORT` (default: from `.env`, else
`3500`).

When `watch` installs a build that changes `tab-deploy.sh` itself, it
restarts on the new version.

## Manual deploys

`./deploy.sh` still works and uses the same `scripts/build-bundle.sh`, so the
binary is built identically. Use it for first-time setup, or to ship a local
build without waiting for CI. The watcher only acts when the release changes,
so it leaves a manual deploy running until the next green push to `main`
replaces it.

To re-publish `main` without a new commit, run the CI workflow by hand from
the Actions tab (**Run workflow**).

## Troubleshooting

- **Nothing deploys**: check the CI run on `main` is green and its
  _Publish tablet build_ job ran. Then run `~/lumina/tab-deploy.sh` on the
  tablet and read its output, or `logs/deploy.log`.
- **"could not fetch the release"**: the tablet has no internet, or no build
  has been published yet. It keeps the current version running.
- **A build was rolled back**: the server would not start on it, usually a
  missing `.env` setting or a startup crash. `logs/server.log` says why. Push
  a fix; the next green build deploys normally.
- **Deploy a skipped build anyway**: `rm ~/lumina/.deploy/last-release` and
  run `~/lumina/tab-deploy.sh`.
