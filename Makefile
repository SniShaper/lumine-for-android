# Lumine Android Build Makefile

GOMOBILE=gomobile
TARGET=android
OUT_DIR=android/app/libs
AAR_NAME=LumineCore.aar
PKG=./mobile
GOFLAGS_BIND=-mod=mod
ANDROID_API=24
POWERSHELL=powershell -ExecutionPolicy Bypass -File

ANDROID_HOME?=$(ANDROID_SDK_ROOT)
ANDROID_SDK_ROOT?=$(ANDROID_HOME)

.PHONY: all android clean

all: android

android:
	$(POWERSHELL) scripts/gomobile-bind.ps1 -AndroidHome "$(ANDROID_HOME)" -Output "$(OUT_DIR)/$(AAR_NAME)" -AndroidApi $(ANDROID_API) -Package "$(PKG)"

clean:
	rm -rf $(OUT_DIR)/$(AAR_NAME) $(OUT_DIR)/LumineCore-sources.jar
