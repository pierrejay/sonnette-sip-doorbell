# Deployment Guide

## Starting Point

This guide assumes you already have:

1. A configured Twilio account, phone number, SIP Domain and TwiML Bin:
   [twilio.md](twilio.md)
2. A freshly flashed Sonnette Linux image:
   [hardware.md](hardware.md#2-linux-image--bsp)

Then follow this guide to build and install the application on the board.

## Deployment Transport

The first-boot deployment path deliberately uses ADB over USB. The
Luckfox/Rockchip SDK enables ADB by default, and on this board it is the
most practical way to get a root shell and push files before networking
exists: WiFi is not configured yet, there is no Ethernet port, and the
UART console is only a recovery fallback.

ADB is convenient during bring-up, but it is also an unauthenticated USB
root shell. This flow keeps it enabled until deployment and verification
are done, then closes it during the final production lock-down step. If
you later need USB ADB again for debugging, re-enable it over SSH after
WiFi is working.

Install `adb` on the development machine that is physically connected to
the board over USB:

```bash
# macOS
brew install android-platform-tools

# Debian/Ubuntu
sudo apt install adb
```

Then connect the board over USB and check that it is visible:

```bash
adb devices
```

## Contents

1. [Build](#1-build)
2. [Fresh image sanity check](#2-fresh-image-sanity-check)
3. [Deploy](#3-deploy)
4. [Harden](#4-harden-after-first-boot)
5. [Run](#5-run)
6. [Final checks and production lock-down](#6-final-checks-and-production-lock-down)
7. [Reference](#7-reference)

## 1. Build

The `build-app` Makefile command cross-compiles both pjsua and the
sonnette Go app. It only builds local artifacts; it does not talk to the
board.

```bash
make build-app SDK_ROOT=/path/to/luckfox-lyra-sdk
```

Outputs:

- `sonnette-arm7`
- `pjsua/pjsua-arm-buildroot-linux-gnueabihf`

Build dependencies: Go >= 1.25 (see `sonnette/go.mod`), and the
Buildroot toolchain produced by the Luckfox Lyra SDK build (used to
cross-compile pjsua against the target sysroot). See
[../pjsua/README.md](../pjsua/README.md) for build prerequisites.

## 2. Fresh image sanity check

Before deploying sonnette, boot the freshly flashed Linux image and make
sure the BSP exposes the audio hardware correctly. These checks do **not**
require `/etc/asound.conf` yet; they talk directly to the ALSA hardware
devices.

```bash
make sanity-audio-adb
```

This is a guided, manual check, not a black-box test. The target pushes
`deploy/dingdong.wav`, lists ALSA devices, asks you to confirm the cards,
plays the chime directly on `hw:1,0`, sets the PDM gain, asks you to speak
for a short recording from `hw:0,0`, and then plays the capture back.

The check is OK if you hear the chime and then hear your recorded voice.
If it fails, debug the image/audio wiring before deploying the app.
`deploy/asound.conf` is installed during deployment and makes the default
PCM convenient for pjsua, but it should not be needed for these direct
hardware checks.

## 3. Deploy

Deploy over ADB. At this stage WiFi may not be configured yet, so do not
assume SSH/SCP works.

Prepare the two credential files locally first:

- `deploy/etc/sonnette.conf` from `deploy/sonnette.conf.example`
- `deploy/etc/wpa_supplicant.conf` from `deploy/wpa_supplicant.conf.example`

`deploy/etc/` is gitignored so real secrets stay local.

```bash
make prepare-secrets
# edit both files before continuing
```

Deploy the files:

```bash
make deploy-adb
```

`deploy-adb` pushes existing binaries/config/scripts/assets only. It
does **not** build anything. If `sonnette-arm7` or
`pjsua/pjsua-arm-buildroot-linux-gnueabihf` is missing, it fails and asks
you to run `make build-app`.

On a freshly flashed board, install the one-time init supervision line
after deploying files:

```bash
make first-boot-adb
```

`first-boot-adb` only adds the `inittab` respawn line if it is not already
present, then reloads init. It does not deploy files and does not rebuild
binaries.

For later software/config updates on an already provisioned board, use
the repeatable deploy target:

```bash
make build-app SDK_ROOT=/path/to/luckfox-lyra-sdk   # only if code/pjsua changed
make deploy-adb
```

`deploy-adb` does not touch `inittab` and does not rerun hardening. The
details are deliberately visible in the Makefile; use
`make -n deploy-adb` to print the exact ADB commands without executing
them.

After WiFi/SSH is configured, the same target paths can be used over the
network with `scp`/`ssh` instead of ADB. The Makefile currently keeps ADB
as the canonical first-boot path because it works before networking is
configured.

## 4. Harden (after first boot)

Run hardening once the files are deployed and the init supervision path is
installed:

```bash
make harden-adb SSH_PUBKEY=~/.ssh/id_ed25519.pub
```

This installs your SSH key, disables password authentication, randomizes
the root password, and fixes credential file permissions. If you don't
have an SSH key yet, generate one before running the target.

**Save the printed root password** — it's the only way to log in
if you lose your SSH key.

## 5. Run

The automatic runtime path installed by `make first-boot-adb` has two
pieces working together:

1. **inittab line** — `init` supervises sonnette. Respawns on crash,
   but skips quickly while `/tmp/sonnette.disable` exists (polling
   pattern).
2. **S99sonnette script** — thin wrapper that toggles the disable flag
   and gives classic `{start|stop|restart|status}` commands.

If you followed step 3, both pieces are already installed over ADB. Only
append `inittab.fragment` once on a freshly flashed image; after that,
normal redeploys should use `make deploy-adb`, which updates the
binaries/config without touching the respawn line.

Credentials are read directly from `/etc/sonnette.conf` (configured via
`credentials_file` in the YAML). No need to source or export anything.

### Logs

- Sonnette's own log goes to syslog via `logger -t sonnette`
  (see `/var/log/messages`). BusyBox `syslogd` already rotates
  `/var/log/messages` by size — usually nothing more to configure.
- Pjsua writes its full log directly to `/var/log/pjsua.log`
  (captured via sonnette, line-buffered via `stdbuf`). Step 3 installs
  a `logrotate` rule at `/etc/logrotate.d/sonnette`, using
  `copytruncate` — safe because sonnette restarts pjsua and reopens the
  file on each cycle.
  This requires the `logrotate` package in the rootfs
  (`BR2_PACKAGE_LOGROTATE=y`; already enabled in
  `bsp/rockchip_rk3506_sonnette_defconfig`) and a periodic runner
  (`cron`, `/etc/periodic`, or equivalent) to invoke `logrotate`.
  If your image does not run it automatically, either add a simple daily
  call to `logrotate /etc/logrotate.conf`, run it manually during
  testing, or reduce/disable pjsua logging.
  The default `pjsua_log_level: 3` keeps useful registration/call events
  without SIP packet trace. Use `2` for quieter production logs, and
  `4+` only temporarily while debugging SIP/media issues.

## 6. Final checks and production lock-down

Runtime smoke checks and diagnostics once everything is deployed and
configured:

```bash
make verify-adb
```

This plays the deployed chime through the default ALSA device, checks
WiFi association, the init wrapper, the `/health` endpoint, pjsua
registration logs, and sonnette syslog output. It does not replace the
fresh-image hardware audio sanity check from step 2, which talks directly
to `hw:0,0` and `hw:1,0`.

End-to-end validation is still physical: press the button and your phone
should ring within a couple of seconds.

Before mounting the board outdoors, close the development access paths
you no longer want exposed:

```bash
make disable-usb-adb
make disable-uart-login-adb
make disable-avahi-adb
make firewall-adb
```

The first two commands are the important physical-access hardening steps:
USB ADB is an unauthenticated root shell, and UART login is useful during
bring-up but not needed once SSH works. Kernel logs still remain visible
on UART after `disable-uart-login-adb`.

If you disable both USB ADB and UART login, SSH becomes the only normal
maintenance path. If WiFi/SSH breaks afterwards, recovery is to remove the
board, enter Maskrom mode, and reflash the image as described in
[hardware.md](hardware.md#flash-the-board). For this single-purpose
device, that trade-off is acceptable in production.

At this point the doorbell should be deployed, locked down, and functional!
The rest of this guide is reference material for configuration and
troubleshooting.

## 7. Reference

The install flow above is enough to build, deploy and validate the
doorbell. The sections below document the runtime configuration,
audio/SIP details, WiFi behavior, monitoring endpoint and optional
production hardening details.

### Configuration

Sonnette uses two config files:

#### sonnette.yaml — settings (non-secrets)

Application parameters. Lives at `/etc/sonnette.yaml`.
See `deploy/sonnette.yaml.example` for a documented template.

```yaml
gpio_pin: 0
dingdong_wav: "/root/dingdong.wav"
snd_clock_rate: 16000
cooldown_sec: 30
max_calls_per_hour: 10
health_port: 8080           # HTTP health endpoint (0 = disabled)
pjsua_log_level: 3           # 2 = quieter, 3 = operational info, 4+ = debug
credentials_file: "/etc/sonnette.conf"  # path to secrets
```

#### sonnette.conf — credentials (secrets)

Shell-style `KEY="value"` file read directly by the sonnette binary at
startup (no need to `source` or `export` — the Go code parses it).

See `deploy/sonnette.conf.example` for a documented template with all
required variables and instructions on where to get them (Twilio
console).

```bash
# Example — do NOT commit real values
TWILIO_ACCOUNT_SID="ACxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
SIP_PASSWORD="changeme"
# ... see sonnette.conf.example for the full list
```

**Important:** `chmod 0600 /etc/sonnette.conf` — credentials must not
be world-readable.

#### Priority order

Credentials file > YAML file > built-in defaults.

In practice, put settings in `sonnette.yaml` and secrets in
`sonnette.conf`.

### ALSA configuration

The RK3506 audio peripherals have two channel quirks:

- PDM capture only accepts 2 channels.
- SAI1 playback only accepts 2 channels.

pjsua opens audio in mono by default, so without a wrapping ALSA
config, mono playback would get a "helium" pitch shift and mono capture
could underflow.

The fix is split between ALSA and pjsua:

- `speaker_dup` (defined in `deploy/asound.conf`) duplicates mono
  playback to left and right.
- pjsua uses `--stereo` so capture opens in native 2-channel mode.
- pjsua handles resampling between 8 kHz SIP PCMU and 16 kHz hardware.

ALSA devices on the board:

| Card | Device | Type | Usage |
|------|--------|------|-------|
| 0 | `hw:0,0` | PDM mic | Stereo capture @ 16 kHz |
| 1 | `hw:1,0` | SAI1 I2S | Stereo playback via MAX98357A |

`deploy/asound.conf` is installed in step 3. The step 2 sanity check
talks directly to `hw:0,0` / `hw:1,0` and does not need this config.

**BSP prerequisites** (full kernel/Buildroot table in
[hardware.md § Kernel Config](hardware.md#kernel-config)):

- Kernel: `CONFIG_SND_SOC_MAX98357A=y` — enabled by
  `bsp/rk3506_sonnette_kernel.config`.
- Kernel: `CONFIG_SND_SOC_ROCKCHIP_PDM_V2`, `CONFIG_SND_SOC_ROCKCHIP_SAI`,
  `CONFIG_SND_SIMPLE_CARD`, `CONFIG_SND_SOC_DUMMY_CODEC` — **stock** in
  `rk3506_luckfox_defconfig`.
- Buildroot: `alsa-utils` (stock) for `aplay` / `arecord` / `amixer`.

### pjsua runtime parameters

Sonnette starts pjsua with these audio/security parameters in `sip.go`:

```text
--config-file /run/pjsua-account.cfg
--stereo
--clock-rate 8000
--snd-clock-rate 16000
--dis-codec .*
--add-codec pcmu
--no-vad --ec-tail 0
--use-tls
--no-udp
--no-tcp
--use-srtp 1
--srtp-secure 1
--stun-srv global.stun.twilio.com
--auto-answer 200
```

SIP credentials are written to `/run/pjsua-account.cfg` with mode
`0600` instead of being passed on the command line. This keeps the SIP
password out of `ps` and `/proc/<pid>/cmdline`.

TLS protects SIP signaling on port 5061. SRTP encrypts media using
SDES key exchange; validated crypto suite: `AES_CM_128_HMAC_SHA1_80`.

Getting SRTP to work with Twilio required three things:

1. pjsua compiled with OpenSSL (`--with-ssl`).
2. UDP and TCP disabled in pjsua (`--no-udp --no-tcp`) so Twilio uses
   the TLS Contact (`sip:...@IP:5061;transport=tls`) instead of falling
   back to UDP on 5060.
3. Twilio SIP Domain → Secure Media enabled, and a TwiML SIP URI with
   `;transport=tls;secure=true`.

Without `--no-udp`, Twilio can silently fall back to UDP signaling and
plain RTP even when `--use-tls` and Secure Media are enabled.

**BSP prerequisites:**

- pjsua binary built against an OpenSSL-enabled toolchain — the
  Makefile in `pjsua/` enforces this (`make check-toolchain`); see
  [../pjsua/README.md](../pjsua/README.md).
- Buildroot: `BR2_PACKAGE_COREUTILS=y` and `BR2_PACKAGE_LIBOPENSSL=y`
  are **already enabled** in `bsp/rockchip_rk3506_sonnette_defconfig`.
  `coreutils` provides `stdbuf`; OpenSSL provides `libssl`/`libcrypto`
  for TLS + SRTP.
- Kernel: nothing specific beyond a working network stack — TLS and
  SRTP are entirely userspace inside pjsua.

### WiFi setup

The board uses a TP-Link TL-WN725N USB dongle (Realtek RTL8188EU,
2.4 GHz only) — see [hardware.md § WiFi](hardware.md#wifi) for the
hardware reference.

**BSP prerequisites** (all in place if the image was built from
`docs/hardware.md`):

- Kernel: `CONFIG_R8188EU=m`, `CONFIG_CFG80211=m`, `CONFIG_MAC80211=m`
  — **stock** in `rk3506_luckfox_defconfig`.
- Buildroot: `BR2_PACKAGE_LINUX_FIRMWARE_RTL_81XX=y` (firmware blob)
  and `BR2_PACKAGE_WPA_SUPPLICANT_WEXT=y` — both **already enabled** in
  `bsp/rockchip_rk3506_sonnette_defconfig`.

The staging driver only exposes `wext` (not `nl80211`), so
`deploy/S90wifi` uses `wpa_supplicant -D wext`. `make deploy-adb` pushes
both `/etc/wpa_supplicant.conf` and the `S90wifi` init script.

`S90wifi` runs before `S99sonnette`, so the network should be ready
before sonnette starts. `make verify-adb` checks the resulting
`wpa_state` and IP address.

Troubleshooting:

| Symptom | Cause | Fix |
|---------|-------|-----|
| `lsusb` does not show `0bda:8179` | USB port or dongle issue | Check `dmesg` and USB power |
| `wlan0` missing after `modprobe r8188eu` | Driver not binding | `rmmod r8188eu && modprobe r8188eu` |
| `wpa_supplicant: Unsupported driver wext` | wpa_supplicant built without wext | Enable `BR2_PACKAGE_WPA_SUPPLICANT_WEXT=y`, rebuild |
| `wpa_state=SCANNING` loops | Wrong SSID/psk, or 5 GHz network | Verify config and scan with `wpa_cli` |

### Monitoring

Sonnette exposes a health check HTTP endpoint so a home server can
detect when the doorbell goes down.

```
GET http://<sonnette-ip>:8080/health
```

```json
{
  "status": "ok",
  "uptime": "2h34m12s",
  "pjsua": true,
  "wifi": "COMPLETED",
  "last_button_press": "2026-05-03T08:14:22Z",
  "last_call_initiated": "2026-05-03T08:14:25Z",
  "last_call_sid": "CAxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
}
```

| Field | Meaning |
|-------|---------|
| `status` | Always `"ok"` if the daemon is responding |
| `uptime` | Time since sonnette started |
| `pjsua` | `true` if the SIP process is alive (calls will work) |
| `wifi` | wpa_supplicant state: `COMPLETED` = connected, `SCANNING`, `DISCONNECTED`... |
| `last_button_press` | RFC 3339 UTC timestamp of the last button press (any press, including ones blocked by cooldown). Omitted until first press. |
| `last_call_initiated` | RFC 3339 UTC timestamp of the last successful Twilio call. Omitted if no call has gone through yet. |
| `last_call_sid` | Twilio Call SID returned by the REST API for the last call. Useful to correlate with Twilio's call logs. Omitted until first call. |

The port is configurable via `health_port` in `sonnette.yaml` (default
`8080`, set to `0` to disable). If you deploy a firewall, whitelist
this port from your monitoring server only.

Example cron on your home server (every 2 minutes):

```bash
*/2 * * * * curl -sf http://192.168.0.225:8080/health | jq -e .pjsua > /dev/null || echo "sonnette down" | mail -s "Alert" you@example.com
```

#### Outbound event webhook (push)

The `/health` endpoint is pull-based: a home server has to poll. For
push-style notifications (someone rang, a call just started, periodic
heartbeat), point `event_webhook_url` at an HTTP endpoint of your
choice. Sonnette sends a JSON POST per event, fire-and-forget, with a
5 s timeout. If the webhook fails, the doorbell call itself is
unaffected — the webhook is best-effort notification.

```yaml
# sonnette.yaml
event_webhook_url: ""               # disabled by default
event_webhook_heartbeat_sec: 0      # 0 = no periodic heartbeat
```

If the URL carries a secret token in its path, prefer setting
`EVENT_WEBHOOK_URL` in `/etc/sonnette.conf` (chmod 0600) instead of
the YAML.

Events emitted (every payload starts with `{"event":..., "ts":"<RFC 3339 UTC>"}`):

| Event | When | Extra fields |
|-------|------|--------------|
| `button_press` | Every press | `outcome`: `calling` \| `cooldown` \| `pjsua_down` (+ `cooldown_remaining` for cooldown) |
| `call_initiated` | Twilio REST call returned 2xx | `sid`: Twilio Call SID |
| `call_failed` | Twilio REST call returned an error | `error`: error message |
| `heartbeat` | Every `event_webhook_heartbeat_sec` seconds (if > 0) | `uptime`, `pjsua`, `wifi` |

Example payload:

```json
{
  "event": "call_initiated",
  "ts": "2026-05-03T08:14:25Z",
  "sid": "CAxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
}
```

### Production hardening

Details for the lock-down commands used in step 6. These are **not needed
during development** (ADB, UART login and Avahi are useful for debugging).

#### Disable ADB (USB root shell)

ADB gives unauthenticated root access over USB. Disable it for
production by renaming its init script and stopping `adbd`:

```bash
make disable-usb-adb
```

To re-enable USB ADB later over SSH:

```bash
make enable-usb-adb-ssh SONNETTE_HOST=192.168.0.225 SSH_KEY=~/.ssh/id_ed25519
```

`SONNETTE_HOST` can be an IP address or hostname reachable over WiFi. The
target renames the init script back and starts it immediately.

#### Disable UART login

The kernel still logs to UART0 (`ttyFIQ0`), but the interactive login
prompt is spawned by a single BusyBox `inittab` getty line. Disable that
prompt in production if the UART pads might be physically reachable:

```bash
make disable-uart-login-adb
```

This comments out the `console::respawn:/sbin/getty ...` line in
`/etc/inittab` and reloads init. Kernel boot logs remain visible on UART;
only the `login:` prompt is removed.

To re-enable UART login while ADB is still available:

```bash
make enable-uart-login-adb
```

#### Disable Avahi/mDNS

Avahi broadcasts `sonnette.local` on the network, making the device
trivially discoverable. Not needed in production:

```bash
make disable-avahi-adb
```

#### Enable firewall

Only do this if you want local packet filtering. Recent Sonnette images
built from this repo already include both sides:

- Buildroot: `BR2_PACKAGE_IPTABLES=y` from
  `bsp/rockchip_rk3506_sonnette_defconfig`
- Kernel: netfilter/conntrack/iptables options from
  `bsp/rk3506_sonnette_kernel.config`

Then deploy `S20firewall`:

```bash
make firewall-adb
```

This drops all inbound traffic by default and whitelists SSH (LAN),
the health check endpoint (LAN), and DHCP. Nothing is opened for SIP
or RTP. SIP signaling uses an outbound TLS connection to Twilio
(5061). SRTP media is separate UDP negotiated in SDP; conntrack keeps
the return path open once pjsua sends media.
