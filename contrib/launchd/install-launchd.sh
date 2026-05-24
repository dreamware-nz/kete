#!/bin/sh
# install-launchd.sh — render kete.proxy.plist.template into
# ~/Library/LaunchAgents/, load it, and verify the proxy is up.
#
# macOS only. Run from the kete repo root or pass the script's path.
#
# Usage:
#   sh contrib/launchd/install-launchd.sh
#
# Honours these env vars (set them once before running):
#   KETE_PORT            default 8765
#   KETE_UPSTREAM        bedrock | cc-proxy | anthropic; default bedrock
#   KETE_DRIFT_MODEL     extraction model id; defaults match upstream
#   AWS_REGION           default $AWS_REGION or us-west-2
#   AWS_PROFILE          (optional)
#   KETE_CC_PROXY_KEY    (cc-proxy only)
#   ANTHROPIC_API_KEY    (anthropic only)
#
# By default the script wires capture-row enrichment to loop back
# through this proxy (KETE_ANTHROPIC_URL=http://127.0.0.1:$PORT). The
# extractor sets x-kete-bypass on every request so the proxy
# short-circuits capture+injection on the loop.

set -eu

if [ "$(uname -s)" != "Darwin" ]; then
  echo "install-launchd.sh: macOS only" >&2
  exit 1
fi

if ! command -v kete >/dev/null 2>&1; then
  echo "install-launchd.sh: 'kete' not on PATH; install first" >&2
  exit 1
fi

KETE_BIN=$(command -v kete)
HOME_DIR="$HOME"
LABEL="kete.proxy"
PLIST="$HOME/Library/LaunchAgents/${LABEL}.plist"
LOG_DIR="$HOME/.kete"

KETE_PORT="${KETE_PORT:-8765}"
KETE_UPSTREAM="${KETE_UPSTREAM:-bedrock}"
AWS_REGION="${AWS_REGION:-us-west-2}"

# Pick a sensible default extraction model for the chosen upstream.
# The user can override KETE_DRIFT_MODEL before running.
case "$KETE_UPSTREAM" in
  bedrock)  default_drift_model="us.anthropic.claude-haiku-4-5-20251001-v1:0" ;;
  *)        default_drift_model="claude-haiku-4-5-20251001" ;;
esac
KETE_DRIFT_MODEL="${KETE_DRIFT_MODEL:-$default_drift_model}"

mkdir -p "$LOG_DIR"

# Build per-upstream env block.
case "$KETE_UPSTREAM" in
  bedrock)
    extra_env="
    <key>AWS_REGION</key>
    <string>${AWS_REGION}</string>"
    if [ -n "${AWS_PROFILE:-}" ]; then
      extra_env="$extra_env
    <key>AWS_PROFILE</key>
    <string>${AWS_PROFILE}</string>"
    fi
    ;;
  cc-proxy)
    if [ -z "${KETE_CC_PROXY_KEY:-}" ]; then
      echo "KETE_UPSTREAM=cc-proxy needs KETE_CC_PROXY_KEY in your env" >&2
      exit 1
    fi
    extra_env="
    <key>KETE_CC_PROXY_KEY</key>
    <string>${KETE_CC_PROXY_KEY}</string>"
    ;;
  anthropic)
    if [ -z "${ANTHROPIC_API_KEY:-}" ]; then
      echo "KETE_UPSTREAM=anthropic needs ANTHROPIC_API_KEY in your env" >&2
      exit 1
    fi
    extra_env="
    <key>ANTHROPIC_API_KEY</key>
    <string>${ANTHROPIC_API_KEY}</string>"
    ;;
  *)
    echo "unknown KETE_UPSTREAM=${KETE_UPSTREAM} (want bedrock|cc-proxy|anthropic)" >&2
    exit 1
    ;;
esac

cat > "$PLIST" <<PLIST_EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN"
  "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>
  <string>${LABEL}</string>
  <key>ProgramArguments</key>
  <array>
    <string>${KETE_BIN}</string>
    <string>proxy</string>
  </array>
  <key>EnvironmentVariables</key>
  <dict>
    <key>HOME</key>
    <string>${HOME_DIR}</string>
    <key>PATH</key>
    <string>${HOME_DIR}/.local/bin:/opt/homebrew/bin:/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin</string>
    <key>KETE_PORT</key>
    <string>${KETE_PORT}</string>
    <key>KETE_UPSTREAM</key>
    <string>${KETE_UPSTREAM}</string>${extra_env}
    <key>KETE_ANTHROPIC_URL</key>
    <string>http://127.0.0.1:${KETE_PORT}</string>
    <key>KETE_DRIFT_MODEL</key>
    <string>${KETE_DRIFT_MODEL}</string>
  </dict>
  <key>RunAtLoad</key>
  <true/>
  <key>KeepAlive</key>
  <dict>
    <key>SuccessfulExit</key>
    <false/>
    <key>Crashed</key>
    <true/>
  </dict>
  <key>ThrottleInterval</key>
  <integer>10</integer>
  <key>StandardOutPath</key>
  <string>${LOG_DIR}/proxy.out.log</string>
  <key>StandardErrorPath</key>
  <string>${LOG_DIR}/proxy.err.log</string>
  <key>WorkingDirectory</key>
  <string>${HOME_DIR}</string>
</dict>
</plist>
PLIST_EOF

# Validate.
plutil "$PLIST" >/dev/null

# Reload.
launchctl bootout "gui/$(id -u)" "$PLIST" 2>/dev/null || true
launchctl bootstrap "gui/$(id -u)" "$PLIST"
launchctl kickstart -k "gui/$(id -u)/${LABEL}"

# Wait briefly + verify.
i=0
while [ $i -lt 20 ]; do
  if curl -fsS "http://127.0.0.1:${KETE_PORT}/health" >/dev/null 2>&1; then
    echo "kete: proxy listening on 127.0.0.1:${KETE_PORT}"
    echo "  plist:  $PLIST"
    echo "  logs:   $LOG_DIR/proxy.{out,err}.log"
    echo "  stop:   launchctl bootout gui/\$(id -u)/${LABEL}"
    echo "  status: launchctl list ${LABEL}"
    exit 0
  fi
  sleep 0.5
  i=$((i+1))
done
echo "kete: did not respond on /health within 10s; check $LOG_DIR/proxy.err.log" >&2
exit 1
