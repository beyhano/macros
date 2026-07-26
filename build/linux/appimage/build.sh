#!/usr/bin/env bash
# Copyright (c) 2018-Present Lea Anthony
# SPDX-License-Identifier: MIT

# Fail script on any error
set -euxo pipefail

# Define variables
APP_DIR="${APP_NAME}.AppDir"

# Create AppDir structure
mkdir -p "${APP_DIR}/usr/bin"
cp -r "${APP_BINARY}" "${APP_DIR}/usr/bin/"
cp -r "${ICON_PATH}" "${APP_DIR}/${APP_NAME}.png"
cp "${DESKTOP_FILE}" "${APP_DIR}/"

if [[ $(uname -m) == *x86_64* ]]; then
    # Download linuxdeploy and make it executable
    wget -q -4 -N https://github.com/linuxdeploy/linuxdeploy/releases/download/continuous/linuxdeploy-x86_64.AppImage
    chmod +x linuxdeploy-x86_64.AppImage

    # Remove the gtk plugin (gtk-4.0 modules not available on this distro)
    rm -f "${APP_DIR}/../linuxdeploy-plugin-gtk.sh" 2>/dev/null || true
    # Prevent linuxdeploy from re-downloading it
    export DISABLE_PLUGINS=1

    # Run linuxdeploy to bundle the application
    ./linuxdeploy-x86_64.AppImage --appdir "${APP_DIR}" --output appimage
    unset DISABLE_PLUGINS
else
    # Download linuxdeploy and make it executable (arm64)
    wget -q -4 -N https://github.com/linuxdeploy/linuxdeploy/releases/download/continuous/linuxdeploy-aarch64.AppImage
    chmod +x linuxdeploy-aarch64.AppImage

    # Run linuxdeploy to bundle the application (arm64)
    ./linuxdeploy-aarch64.AppImage --appdir "${APP_DIR}" --output appimage
fi

# Rename the generated AppImage (handle both with and without arch suffix)
for f in "${APP_NAME}"*.AppImage; do
  [ -f "$f" ] && mv "$f" "${APP_NAME}.AppImage" && break
done 2>/dev/null || true

