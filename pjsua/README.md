# pjsua build

Cross-compilation of `pjsua` (the SIP client used at runtime by sonnette)
for the target board. The Makefile downloads pjproject 2.14, configures
it against the Buildroot toolchain, and produces a binary that links
dynamically against the target sysroot's libc, ALSA and OpenSSL.

## Why the SDK toolchain (and not just `gcc-arm-linux-gnueabihf`)

The compiler itself (`arm-buildroot-linux-gnueabihf-gcc`) is just
upstream GCC. What matters is the **sysroot** it points at. pjsua links
against three target libraries:

- **glibc** — Linux + libc ABI
- **ALSA** (`libasound`) — for the PDM mic and I2S speaker
- **OpenSSL** (`libssl` + `libcrypto`) — for SIP/TLS and SRTP

The Buildroot toolchain produced by the SDK has these libraries
**already installed in its sysroot**, with the exact versions present
in the rootfs that gets flashed on the board. Using a generic Debian
`gcc-arm-linux-gnueabihf` works in principle, but you'd then have to
provide your own sysroot with matching ALSA + OpenSSL — extra work
with no benefit if you're targeting this board.

→ For this repo, use the SDK Buildroot toolchain. It's the simplest
path and matches the shipped rootfs.

## Prerequisites

Build the Luckfox Lyra SDK at least once:

```bash
cd /path/to/luckfox-lyra-sdk
./build.sh lunch     # pick the sonnette config
./build.sh
```

This populates `buildroot/output/<config>/host/` with the toolchain and
the populated sysroot.

## Build

```bash
cd pjsua/
make build SDK_ROOT=/path/to/luckfox-lyra-sdk
```

Defaults that you can override:

| Variable    | Default                              | Notes |
|-------------|--------------------------------------|-------|
| `SDK_ROOT`  | `$HOME/luckfox-lyra-sdk`             | Where the SDK was checked out |
| `BR_OUTPUT` | `rockchip_rk3506_sonnette`           | Buildroot output directory name (matches the project's defconfig in `bsp/`) |
| `CROSS`     | `arm-buildroot-linux-gnueabihf`      | Cross-compile triplet |

The strict `check-toolchain` step verifies that the toolchain and the
needed sysroot libraries (ALSA headers, libasound, OpenSSL headers) are
present before launching the configure/compile cycle. If anything is
missing it tells you exactly which path it expected.

## Output

`pjsua/pjsua-arm-buildroot-linux-gnueabihf` — ELF 32-bit ARM EABI5,
dynamically linked (glibc, libasound, libssl, libcrypto from the
sysroot), hard-float. Around 1.7 MB. Copy it to the target as
`/usr/bin/pjsua` (see [`docs/deploy.md`](../docs/deploy.md)).

Sanity check on the host:
```bash
file pjsua-arm-buildroot-linux-gnueabihf
# → ELF 32-bit LSB executable, ARM, EABI5 ...
```

On the board:
```bash
pjsua --version    # prints pjsua / pjsip versions
```

Runtime log verbosity is controlled by sonnette's `pjsua_log_level`
setting, which is passed to pjsua as `--log-level`. The default is `3`
for registration/call-state visibility without SIP packet trace. Use
`2` for quieter production logs and `4+` only for short debugging
sessions.

## pjproject version

Pinned to **2.14**. Newer versions may compile but haven't been tested
with the TLS+SRTP flow against Twilio.

## Build flags rationale

```
--with-ssl=$(SYSROOT)/usr   # required for TLS (SIP signaling) + SRTP
--disable-video             # no video here
--disable-libwebrtc         # uses SSE2 (x86), incompatible with ARMv7
--disable-*-codec           # keep only PCMU (the codec Twilio uses)
--enable-shared=no          # build pjsip libs as static archives
                            # (the final pjsua binary still links
                            #  dynamically against libc/ALSA/SSL)
```

## Building for a non-RK3506 Linux board

If you're porting this to a different ARM Linux SBC, the Makefile is
still useful as a template — you just need to point `SDK_ROOT`,
`BR_OUTPUT` and `CROSS` at your own toolchain layout, **and** make sure
that toolchain's sysroot has ALSA + OpenSSL installed.

The simplest paths are:
- A Buildroot or Yocto-built toolchain for your board (recommended —
  same approach, different SDK).
- A Debian/Ubuntu armhf cross-toolchain (`gcc-arm-linux-gnueabihf`)
  with libasound2-dev + libssl-dev installed for the armhf
  architecture (via `dpkg --add-architecture` or a debootstrap chroot).

For non-ARMv7 targets (aarch64 Pi 4/5, x86 SBCs), the `CROSS` triplet
and pjsip's `--host`/`--target` change accordingly. There's nothing
rk3506-specific in pjsua itself — only the toolchain wiring.
