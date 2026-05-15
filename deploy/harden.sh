#!/bin/sh
# ============================================================
# harden.sh — Security hardening for sonnette after a fresh flash
# ============================================================
# Run via ADB after flashing:
#   adb push deploy/harden.sh /tmp/
#   adb push ~/.ssh/id_ed25519.pub /tmp/authorized_key.pub
#   adb shell "sh /tmp/harden.sh /tmp/authorized_key.pub"
#
# Or via SSH (if you still have access):
#   scp deploy/harden.sh root@sonnette:/tmp/
#   scp ~/.ssh/id_ed25519.pub root@sonnette:/tmp/authorized_key.pub
#   ssh root@sonnette "sh /tmp/harden.sh /tmp/authorized_key.pub"
#
# What it does:
#   1. Installs SSH public key for root
#   2. Hardens sshd (key-only, no password login)
#   3. Randomizes root password (printed once, save it)
#   4. chmod 0600 on credential files
#   5. Restarts sshd
#
# After running, SSH password auth is DISABLED. Keep your private
# key safe — it's the only way in (besides ADB/UART).

set -e

PUBKEY_FILE="$1"

if [ -z "$PUBKEY_FILE" ] || [ ! -f "$PUBKEY_FILE" ]; then
    echo "Usage: $0 <path-to-ssh-public-key>"
    echo "Example: $0 /tmp/authorized_key.pub"
    exit 1
fi

echo "=== Sonnette security hardening ==="

# 1. SSH authorized key
echo "[1/5] Installing SSH public key..."
mkdir -p /root/.ssh
chmod 700 /root/.ssh
cp "$PUBKEY_FILE" /root/.ssh/authorized_keys
chmod 600 /root/.ssh/authorized_keys
echo "      Key: $(cat /root/.ssh/authorized_keys | cut -d' ' -f1-2 | cut -c1-60)..."

# 2. Harden sshd_config
echo "[2/5] Hardening sshd_config..."
sed -i 's/^#*PasswordAuthentication.*/PasswordAuthentication no/' /etc/ssh/sshd_config
sed -i 's/^#*PermitRootLogin.*/PermitRootLogin prohibit-password/' /etc/ssh/sshd_config

# 3. Randomize root password
echo "[3/5] Randomizing root password..."
# Generate a random password using /dev/urandom (works without openssl)
NEW_PASS=$(head -c 12 /dev/urandom | base64 | tr -d '/+=' | head -c 16)
# Generate SHA-256 hash using busybox mkpasswd if available, else openssl
if command -v mkpasswd >/dev/null 2>&1; then
    HASH=$(mkpasswd -m sha-256 "$NEW_PASS")
elif command -v openssl >/dev/null 2>&1; then
    HASH=$(openssl passwd -5 "$NEW_PASS")
else
    echo "      WARNING: no mkpasswd or openssl — password NOT changed"
    HASH=""
fi
if [ -n "$HASH" ]; then
    sed -i "s|^root:[^:]*:|root:${HASH}:|" /etc/shadow
    echo ""
    echo "      ┌──────────────────────────────────────┐"
    echo "      │  NEW ROOT PASSWORD: $NEW_PASS  │"
    echo "      │  Save this somewhere safe.           │"
    echo "      │  SSH is key-only, this is for        │"
    echo "      │  UART/ADB console access only.       │"
    echo "      └──────────────────────────────────────┘"
    echo ""
fi

# 4. Fix file permissions
echo "[4/5] Fixing credential file permissions..."
[ -f /etc/sonnette.conf ] && chmod 0600 /etc/sonnette.conf && echo "      /etc/sonnette.conf → 0600"
[ -f /etc/wpa_supplicant.conf ] && chmod 0600 /etc/wpa_supplicant.conf && echo "      /etc/wpa_supplicant.conf → 0600"

# 5. Restart sshd
echo "[5/5] Restarting sshd..."
killall sshd 2>/dev/null || true
/usr/sbin/sshd
echo "      sshd restarted"

echo ""
echo "=== Hardening complete ==="
echo "Test SSH key auth before closing this session:"
echo "  ssh -i ~/.ssh/id_ed25519 root@$(hostname).local"

# Cleanup
rm -f "$PUBKEY_FILE" /tmp/harden.sh 2>/dev/null
