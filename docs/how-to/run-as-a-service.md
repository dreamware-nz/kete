# How-to: run kete proxy at login (macOS)

`kete proxy` is a server. You don't want to remember to start it
every time you open a terminal. On macOS, the right tool for that
is `launchd`.

This how-to wires `kete proxy` into a per-user `LaunchAgent` so it
starts at login, restarts on crash, and writes its logs somewhere
you can find them.

## What you need

- `kete` on your `PATH` (`which kete` returns a path).
- The upstream you want to use, configured. See:
  - [`use-bedrock.md`](use-bedrock.md) for AWS Bedrock,
  - [`use-cc-proxy.md`](use-cc-proxy.md) for cc-proxy,
  - or have `ANTHROPIC_API_KEY` set for direct API.

## Quick install (Bedrock by default)

From the kete repo:

```sh
KETE_UPSTREAM=bedrock AWS_REGION=us-west-2 \
  sh contrib/launchd/install-launchd.sh
```

That renders a plist into `~/Library/LaunchAgents/kete.proxy.plist`,
loads it, and verifies the proxy is reachable on
`http://127.0.0.1:8765/health` before exiting.

For cc-proxy:

```sh
KETE_UPSTREAM=cc-proxy KETE_CC_PROXY_KEY=... \
  sh contrib/launchd/install-launchd.sh
```

For Anthropic-direct:

```sh
KETE_UPSTREAM=anthropic ANTHROPIC_API_KEY=sk-ant-... \
  sh contrib/launchd/install-launchd.sh
```

Honoured env vars:

| Var                | Default      | Notes                                |
| ------------------ | ------------ | ------------------------------------ |
| `KETE_PORT`        | `8765`       | Proxy listens here                   |
| `KETE_UPSTREAM`    | `bedrock`    | `bedrock` \| `cc-proxy` \| `anthropic` |
| `AWS_REGION`       | `us-west-2`  | Bedrock only                         |
| `AWS_PROFILE`      | (unset)      | Bedrock only, optional               |
| `KETE_CC_PROXY_KEY`| —            | cc-proxy: required                   |
| `ANTHROPIC_API_KEY`| —            | anthropic: required                  |

## What the agent does

- Runs `kete proxy` at login (`RunAtLoad`).
- Restarts on crash, but not on clean exit (`KeepAlive` with
  `Crashed=true, SuccessfulExit=false`).
- 10-second `ThrottleInterval` so a crash loop doesn't hammer the
  scheduler.
- Logs to `~/.kete/proxy.out.log` and `~/.kete/proxy.err.log`.
- Working directory is `$HOME` so kete's project-path resolution
  doesn't anchor on `/`.

## Operating it

```sh
# is it running?
launchctl list kete.proxy

# tail logs
tail -f ~/.kete/proxy.{out,err}.log

# restart
launchctl kickstart -k gui/$(id -u)/kete.proxy

# stop and disable
launchctl bootout gui/$(id -u)/kete.proxy

# fully remove
launchctl bootout gui/$(id -u)/kete.proxy
rm ~/Library/LaunchAgents/kete.proxy.plist
```

## Doing it by hand

If you'd rather edit the plist yourself, copy the template:

```sh
cp contrib/launchd/kete.proxy.plist.template \
   ~/Library/LaunchAgents/kete.proxy.plist
sed -i '' "s|__HOME__|$HOME|g" ~/Library/LaunchAgents/kete.proxy.plist
$EDITOR ~/Library/LaunchAgents/kete.proxy.plist     # set your upstream env
launchctl bootstrap gui/$(id -u) ~/Library/LaunchAgents/kete.proxy.plist
launchctl kickstart -k gui/$(id -u)/kete.proxy
```

The template has commented blocks for each upstream — uncomment the
one you want.

## Pointing your client at it

With the proxy listening on `127.0.0.1:8765`:

```sh
export ANTHROPIC_BASE_URL=http://127.0.0.1:8765
```

…or configure your client (Crush, Claude Code, etc.) to point at it
directly. Crush, specifically: add a provider with
`type: "anthropic"` and `base_url: "http://127.0.0.1:8765"`, then
set the active model to a Bedrock inference-profile id like
`us.anthropic.claude-haiku-4-5-20251001-v1:0`.

## Linux

`launchd` is macOS-only. On Linux, run kete under `systemd --user`:

```ini
# ~/.config/systemd/user/kete-proxy.service
[Unit]
Description=kete proxy
After=network-online.target

[Service]
Type=simple
ExecStart=%h/.local/bin/kete proxy
Restart=on-failure
RestartSec=10
Environment=KETE_PORT=8765
Environment=KETE_UPSTREAM=bedrock
Environment=AWS_REGION=us-west-2

[Install]
WantedBy=default.target
```

```sh
systemctl --user daemon-reload
systemctl --user enable --now kete-proxy
journalctl --user -u kete-proxy -f
```

We don't ship a unit file generator yet; if you want one, file an
idea.
