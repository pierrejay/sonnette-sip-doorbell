# Root workflow helpers for Sonnette.
#
# The Makefile keeps the deploy/build flow repeatable without hiding the
# underlying commands. Override variables on the command line, for example:
#   make sdk-sync SDK_ROOT=/path/to/luckfox-lyra-sdk

SDK_ROOT ?= $(HOME)/luckfox-lyra-sdk
BR_OUTPUT ?= rockchip_rk3506_sonnette
BOARD_DEFCONFIG ?= custom_sonnette_buildroot_spinand

ADB ?= adb
SSH ?= ssh
SONNETTE_HOST ?= sonnette.local
SSH_KEY ?= $(HOME)/.ssh/id_ed25519
SSH_OPTS ?= -i $(SSH_KEY)
GO ?= go
SSH_PUBKEY ?= $(HOME)/.ssh/id_ed25519.pub

SONNETTE_BIN := sonnette-arm7
PJSUA_BIN := pjsua/pjsua-arm-buildroot-linux-gnueabihf

.PHONY: help
help:
	@echo "Sonnette workflow"
	@echo ""
	@echo "Fresh board deploy flow:"
	@echo "  1. make build-app          Build Go daemon + pjsua"
	@echo "  2. make sanity-audio-adb   Guided direct ALSA speaker/mic check"
	@echo "  3. make prepare-secrets    Create deploy/etc templates if missing"
	@echo "  4. make deploy-adb         Push existing binaries/config/scripts/assets"
	@echo "  5. make first-boot-adb     One-time inittab setup after fresh flash"
	@echo "  6. make harden-adb         Push and run deploy/harden.sh"
	@echo "  7. make verify-adb         Runtime smoke checks and diagnostics"
	@echo ""
	@echo "Update flow:"
	@echo "  make build-app             Rebuild binaries if code/pjsua changed"
	@echo "  make deploy-adb            Push existing artifacts/config/scripts/assets"
	@echo "  make verify-adb            Runtime smoke checks and diagnostics"
	@echo ""
	@echo "SDK / image:"
	@echo "  make sdk-sync              Copy BSP files into SDK_ROOT ($(SDK_ROOT))"
	@echo "  make image                 Run SDK lunch + full image build"
	@echo "  make flash                 Flash SDK-built update.img via rkflash.sh"
	@echo ""
	@echo "Application:"
	@echo "  make build-app             Build Go daemon + pjsua"
	@echo "  make build-go              Build sonnette-arm7"
	@echo "  make build-pjsua           Build pjsua with SDK Buildroot sysroot"
	@echo ""
	@echo "Deploy over ADB:"
	@echo "  make prepare-secrets       Create deploy/etc templates if missing"
	@echo "  make deploy-adb            Push existing binaries/config/scripts/assets"
	@echo "  make first-boot-adb        One-time inittab setup after fresh flash"
	@echo "  make harden-adb            Push and run deploy/harden.sh"
	@echo "  make sanity-audio-adb      Guided direct ALSA speaker/mic check"
	@echo "  make verify-adb            Runtime smoke checks and diagnostics"
	@echo "  make firewall-adb          Deploy and start S20firewall"
	@echo "  make disable-avahi-adb     Disable Avahi/mDNS for production"
	@echo "  make disable-usb-adb       Disable USB ADB for production"
	@echo "  make disable-uart-login-adb  Disable UART login prompt for production"
	@echo "  make enable-uart-login-adb   Re-enable UART login prompt for debugging"
	@echo "  make enable-usb-adb-ssh    Re-enable USB ADB over SSH for debugging"
	@echo ""
	@echo "Variables:"
	@echo "  SDK_ROOT=/path/to/sdk      Luckfox/Rockchip SDK root"
	@echo "  BR_OUTPUT=name            Buildroot output dir for pjsua sysroot"
	@echo "  ADB=adb                   ADB executable"
	@echo "  SONNETTE_HOST=sonnette.local  Host/IP for SSH maintenance"
	@echo "  SSH_KEY=~/.ssh/id_ed25519 Private key for SSH maintenance"
	@echo "  SSH_PUBKEY=~/.ssh/id_ed25519.pub"

.PHONY: sdk-sync
sdk-sync:
	@test -d "$(SDK_ROOT)" || { echo "SDK_ROOT not found: $(SDK_ROOT)"; exit 1; }
	mkdir -p "$(SDK_ROOT)/kernel-6.1/arch/arm/boot/dts"
	mkdir -p "$(SDK_ROOT)/kernel-6.1/arch/arm/configs"
	mkdir -p "$(SDK_ROOT)/device/rockchip/.chips/rk3506"
	mkdir -p "$(SDK_ROOT)/buildroot/configs"
	cp bsp/rk3506g-custom-sonnette.dts bsp/rk3506-custom-sonnette.dtsi \
		"$(SDK_ROOT)/kernel-6.1/arch/arm/boot/dts/"
	cp bsp/rk3506_sonnette_kernel.config \
		"$(SDK_ROOT)/kernel-6.1/arch/arm/configs/"
	cp bsp/custom_sonnette_buildroot_spinand_defconfig \
		"$(SDK_ROOT)/device/rockchip/.chips/rk3506/"
	cp bsp/rockchip_rk3506_sonnette_defconfig \
		"$(SDK_ROOT)/buildroot/configs/"
	@echo "BSP synced to $(SDK_ROOT)"
	@echo "Reminder: U-Boot XTX SPI NAND support still requires the manual SDK edit in docs/hardware.md."

.PHONY: image
image: sdk-sync
	cd "$(SDK_ROOT)" && ./build.sh lunch "$(BOARD_DEFCONFIG)" && ./build.sh

.PHONY: flash
flash:
	cd "$(SDK_ROOT)" && sudo ./rkflash.sh update

.PHONY: build-app build-go build-pjsua
build-app: build-go build-pjsua

build-go:
	cd sonnette && GOOS=linux GOARCH=arm GOARM=7 $(GO) build -o ../$(SONNETTE_BIN) .

build-pjsua:
	$(MAKE) -C pjsua build SDK_ROOT="$(SDK_ROOT)" BR_OUTPUT="$(BR_OUTPUT)"

.PHONY: check-artifacts check-secrets
check-artifacts:
	@test -x "$(SONNETTE_BIN)" || { echo "Missing $(SONNETTE_BIN); run make build-app SDK_ROOT=/path/to/sdk"; exit 1; }
	@test -x "$(PJSUA_BIN)" || { echo "Missing $(PJSUA_BIN); run make build-app SDK_ROOT=/path/to/sdk"; exit 1; }

check-secrets:
	@test -f deploy/etc/sonnette.conf || { echo "Missing deploy/etc/sonnette.conf; run make prepare-secrets"; exit 1; }
	@test -f deploy/etc/wpa_supplicant.conf || { echo "Missing deploy/etc/wpa_supplicant.conf; run make prepare-secrets"; exit 1; }

.PHONY: prepare-secrets
prepare-secrets:
	mkdir -p deploy/etc
	@test -f deploy/etc/sonnette.conf || cp deploy/sonnette.conf.example deploy/etc/sonnette.conf
	@test -f deploy/etc/wpa_supplicant.conf || cp deploy/wpa_supplicant.conf.example deploy/etc/wpa_supplicant.conf
	@echo "Edit deploy/etc/sonnette.conf and deploy/etc/wpa_supplicant.conf before deploying."

.PHONY: deploy-adb
deploy-adb: check-artifacts check-secrets
	$(ADB) push $(SONNETTE_BIN) /usr/bin/sonnette
	$(ADB) push $(PJSUA_BIN) /usr/bin/pjsua
	$(ADB) shell "chmod +x /usr/bin/sonnette /usr/bin/pjsua"
	$(ADB) push deploy/sonnette.yaml.example /etc/sonnette.yaml
	$(ADB) push deploy/etc/sonnette.conf /etc/sonnette.conf
	$(ADB) shell "chmod 0600 /etc/sonnette.conf"
	$(ADB) push deploy/asound.conf /etc/asound.conf
	$(ADB) push deploy/etc/wpa_supplicant.conf /etc/wpa_supplicant.conf
	$(ADB) shell "chmod 0600 /etc/wpa_supplicant.conf"
	$(ADB) push deploy/dingdong.wav /root/dingdong.wav
	$(ADB) push deploy/S90wifi /etc/init.d/S90wifi
	$(ADB) push deploy/S99sonnette /etc/init.d/S99sonnette
	$(ADB) shell "chmod +x /etc/init.d/S90wifi /etc/init.d/S99sonnette"
	$(ADB) shell "mkdir -p /etc/logrotate.d"
	$(ADB) push deploy/logrotate.sonnette /etc/logrotate.d/sonnette

.PHONY: first-boot-adb
first-boot-adb:
	$(ADB) push deploy/inittab.fragment /tmp/inittab.fragment
	$(ADB) shell "grep -qF '/usr/bin/sonnette -config /etc/sonnette.yaml' /etc/inittab || cat /tmp/inittab.fragment >> /etc/inittab; rm -f /tmp/inittab.fragment"
	$(ADB) shell "kill -HUP 1"

.PHONY: harden-adb
harden-adb:
	@test -f "$(SSH_PUBKEY)" || { echo "SSH_PUBKEY not found: $(SSH_PUBKEY)"; exit 1; }
	$(ADB) push deploy/harden.sh /tmp/
	$(ADB) push "$(SSH_PUBKEY)" /tmp/authorized_key.pub
	$(ADB) shell "sh /tmp/harden.sh /tmp/authorized_key.pub"

.PHONY: sanity-audio-adb
sanity-audio-adb:
	@echo "Fresh-image ALSA sanity check over ADB"
	@echo ""
	$(ADB) push deploy/dingdong.wav /root/dingdong.wav
	@echo ""
	@echo "1/4 Listing ALSA cards and /dev/snd nodes."
	$(ADB) shell "aplay -l; arecord -l; ls -l /dev/snd"
	@printf "\nExpected: capture card pdmmic on hw:0,0 and playback card i2sspeaker on hw:1,0. Press Enter to play the chime, or Ctrl-C to stop. "; read _
	@echo ""
	@echo "2/4 Playing dingdong.wav on hw:1,0. You should hear it on the speaker."
	$(ADB) shell "aplay -D hw:1,0 /root/dingdong.wav"
	@printf "\nIf you heard the chime, press Enter to continue to the microphone test. "; read _
	@echo ""
	@echo "3/4 Setting PDM microphone gain, then recording 3 seconds from hw:0,0."
	$(ADB) shell "amixer -c 0 cset numid=7 100%"
	@printf "Speak toward the microphone after pressing Enter. "; read _
	$(ADB) shell "arecord -D hw:0,0 -c 2 -r 16000 -f S16_LE -d 3 /tmp/mic-test.wav"
	@echo ""
	@echo "4/4 Playing the captured microphone sample back on hw:1,0."
	$(ADB) shell "aplay -D hw:1,0 /tmp/mic-test.wav"
	@echo ""
	@echo "Sanity check is OK if you heard the chime and then heard your recorded voice."

.PHONY: verify-adb
verify-adb:
	@echo "Runtime smoke checks and diagnostics over ADB"
	@echo ""
	@echo "1/6 Playing deployed chime through the default ALSA device."
	$(ADB) shell "aplay /root/dingdong.wav"
	@echo ""
	@echo "2/6 Checking WiFi association."
	$(ADB) shell "wpa_cli -i wlan0 status | grep -E 'wpa_state|ip_address' || true"
	@echo ""
	@echo "3/6 Checking init wrapper status."
	$(ADB) shell "/etc/init.d/S99sonnette status || true"
	@echo ""
	@echo "4/6 Checking local health endpoint."
	$(ADB) shell "curl -s http://127.0.0.1:8080/health || true"
	@echo ""
	@echo "5/6 Checking recent pjsua registration/call logs."
	$(ADB) shell "grep -E 'registration success|200 OK' /var/log/pjsua.log | tail || true"
	@echo ""
	@echo "6/6 Checking recent sonnette syslog lines."
	$(ADB) shell "grep sonnette /var/log/messages | tail || true"

.PHONY: firewall-adb
firewall-adb:
	$(ADB) push deploy/S20firewall /etc/init.d/S20firewall
	$(ADB) shell "chmod +x /etc/init.d/S20firewall"
	$(ADB) shell "/etc/init.d/S20firewall start"

.PHONY: disable-avahi-adb
disable-avahi-adb:
	$(ADB) shell "[ ! -f /etc/init.d/S50avahi-daemon ] || mv /etc/init.d/S50avahi-daemon /etc/init.d/disabled_S50avahi-daemon"
	$(ADB) shell "killall avahi-daemon 2>/dev/null || true"

.PHONY: disable-usb-adb
disable-usb-adb:
	$(ADB) shell "[ ! -f /etc/init.d/S50usbdevice.sh ] || mv /etc/init.d/S50usbdevice.sh /etc/init.d/disabled_S50usbdevice.sh"
	$(ADB) shell "killall adbd 2>/dev/null || true"

.PHONY: disable-uart-login-adb
disable-uart-login-adb:
	$(ADB) shell "sed -i 's|^console::respawn:/sbin/getty|#&|' /etc/inittab && kill -HUP 1"

.PHONY: enable-uart-login-adb
enable-uart-login-adb:
	$(ADB) shell "sed -i 's|^#console::respawn:/sbin/getty|console::respawn:/sbin/getty|' /etc/inittab && kill -HUP 1"

.PHONY: enable-usb-adb-ssh
enable-usb-adb-ssh:
	$(SSH) $(SSH_OPTS) root@$(SONNETTE_HOST) "if [ -f /etc/init.d/disabled_S50usbdevice.sh ]; then mv /etc/init.d/disabled_S50usbdevice.sh /etc/init.d/S50usbdevice.sh; fi; test -x /etc/init.d/S50usbdevice.sh || { echo 'S50usbdevice.sh not found'; exit 1; }; /etc/init.d/S50usbdevice.sh start"
