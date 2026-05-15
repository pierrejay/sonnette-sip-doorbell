# Cross-compile pjsua for the target board (RK3506 + Buildroot sysroot).
#
# Usage:
#   make download                          # fetch pjproject source
#   make build                             # configure + compile
#   make build SDK_ROOT=/path/to/sdk       # override SDK location
#   make clean                             # clean the build
#
# Prerequisite: the Luckfox Lyra SDK must have been built at least once
# so that the Buildroot toolchain AND the populated sysroot (ALSA,
# OpenSSL) are available. See pjsua/README.md for the rationale.

PJSIP_VERSION := 2.14
PJSIP_DIR     := pjproject-$(PJSIP_VERSION)
PJSIP_TAR     := $(PJSIP_VERSION).tar.gz
PJSIP_URL     := https://github.com/pjsip/pjproject/archive/refs/tags/$(PJSIP_TAR)

# Buildroot toolchain layout (override on the command line if needed).
SDK_ROOT  ?= $(HOME)/luckfox-lyra-sdk
BR_OUTPUT ?= rockchip_rk3506_sonnette
CROSS     ?= arm-buildroot-linux-gnueabihf

TC_HOST   := $(SDK_ROOT)/buildroot/output/$(BR_OUTPUT)/host
SYSROOT   := $(TC_HOST)/$(CROSS)/sysroot

export PATH := $(TC_HOST)/bin:$(PATH)

# Final output binary
PJSUA_BIN := $(PJSIP_DIR)/pjsip-apps/bin/pjsua-$(CROSS)
OUTPUT    := pjsua-$(CROSS)

# Configure flags. ALSA is auto-detected from the sysroot, --with-ssl
# is required for TLS+SRTP, --disable-*-codec keeps only PCMU (Twilio).
CONFIGURE_FLAGS := \
	--host=$(CROSS) \
	--target=$(CROSS) \
	--with-ssl=$(SYSROOT)/usr \
	--disable-video \
	--disable-opencore-amr \
	--disable-g7221-codec \
	--disable-ilbc-codec \
	--disable-speex-codec \
	--disable-speex-aec \
	--disable-gsm-codec \
	--disable-g722-codec \
	--disable-libwebrtc \
	--enable-shared=no

.PHONY: all download build configure compile install clean check-toolchain

all: build

check-toolchain:
	@which $(CROSS)-gcc > /dev/null 2>&1 || { \
		echo "ERROR: cross compiler '$(CROSS)-gcc' not found in PATH."; \
		echo "Expected: $(TC_HOST)/bin/$(CROSS)-gcc"; \
		echo "Has the SDK been built?  cd $(SDK_ROOT) && ./build.sh"; \
		echo "Override with: make build SDK_ROOT=/path/to/sdk BR_OUTPUT=<dir>"; \
		exit 1; }
	@test -f $(SYSROOT)/usr/include/alsa/asoundlib.h || { \
		echo "ERROR: ALSA headers missing in sysroot:"; \
		echo "  $(SYSROOT)/usr/include/alsa/asoundlib.h"; \
		echo "The SDK rootfs must include alsa-lib (it does by default)."; \
		exit 1; }
	@test -f $(SYSROOT)/usr/lib/libasound.so || \
	  test -f $(SYSROOT)/usr/lib/libasound.so.2 || { \
		echo "ERROR: libasound not found in sysroot:"; \
		echo "  $(SYSROOT)/usr/lib/libasound.so[.2]"; \
		exit 1; }
	@test -f $(SYSROOT)/usr/include/openssl/ssl.h || { \
		echo "ERROR: OpenSSL headers missing in sysroot:"; \
		echo "  $(SYSROOT)/usr/include/openssl/ssl.h"; \
		echo "The SDK Buildroot config must enable libopenssl"; \
		echo "(BR2_PACKAGE_LIBOPENSSL=y; BR2_PACKAGE_LIBOPENSSL_BIN=y"; \
		echo "only installs the openssl command-line tool) — required for TLS+SRTP."; \
		exit 1; }
	@echo "Toolchain: $$($(CROSS)-gcc --version | head -1)"
	@echo "Sysroot:   $(SYSROOT)"

download: $(PJSIP_DIR)

$(PJSIP_TAR):
	wget -q --show-progress $(PJSIP_URL)

$(PJSIP_DIR): $(PJSIP_TAR)
	tar xf $(PJSIP_TAR)
	@echo "pjproject $(PJSIP_VERSION) source extracted."

build: check-toolchain $(PJSIP_DIR) configure compile
	cp $(PJSUA_BIN) $(OUTPUT)
	@echo ""
	@echo "Build OK: $(OUTPUT)"
	@file $(OUTPUT)

configure: $(PJSIP_DIR)
	cd $(PJSIP_DIR) && ./configure $(CONFIGURE_FLAGS)

compile: $(PJSIP_DIR)
	cd $(PJSIP_DIR) && make dep && make -j$$(nproc)

clean:
	[ -d $(PJSIP_DIR) ] && cd $(PJSIP_DIR) && make clean || true

distclean:
	rm -rf $(PJSIP_DIR) $(PJSIP_TAR) $(OUTPUT)
