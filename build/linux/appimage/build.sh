#!/usr/bin/env bash
set -euo pipefail

APP_DIR="${APP_NAME}.AppDir"

# Prepare AppDir
mkdir -p "${APP_DIR}/usr/bin"
cp -r "${APP_BINARY}" "${APP_DIR}/usr/bin/"
cp -r "${ICON_PATH}" "${APP_DIR}/${APP_NAME}.png"
cp "${DESKTOP_FILE}" "${APP_DIR}/"

# Create AppRun
cat > "${APP_DIR}/AppRun" << 'EOF'
#!/bin/bash
APPDIR="$(dirname "$(readlink -f "$0")")"
exec "${APPDIR}/usr/bin/macros"
EOF
chmod +x "${APP_DIR}/AppRun"

# Download linuxdeploy
wget -q -4 -N https://github.com/linuxdeploy/linuxdeploy/releases/download/continuous/linuxdeploy-x86_64.AppImage
chmod +x linuxdeploy-x86_64.AppImage

# Extract linuxdeploy to get appimagetool
./linuxdeploy-x86_64.AppImage --appimage-extract >/dev/null 2>&1
APPIMAGE_TOOL=$(find squashfs-root -name appimagetool -type f | head -1)

# Extract the appimagetool for later use
cp "$APPIMAGE_TOOL" appimagetool

# Clean up linuxdeploy
rm -rf squashfs-root linuxdeploy-x86_64.AppImage linuxdeploy-plugin-*.sh 2>/dev/null || true

# Bundle libraries manually using ldd + copy
bundle_libs() {
  local bin="$1"
  local dest="$2"
  local processed=""
  local todo="$bin"
  
  while [ -n "$todo" ]; do
    local current=""
    for f in $todo; do
      echo "$f" >> /tmp/processed_$$.txt 2>/dev/null || true
      if ldd "$f" 2>/dev/null; then
        current="$current $(ldd "$f" 2>/dev/null | awk '/=> \//{print $3}' | sort -u)"
      fi
    done
    todo=""
    for lib in $current; do
      if [ -f "$lib" ] && ! grep -qF "$lib" /tmp/processed_$$.txt 2>/dev/null; then
        mkdir -p "$dest/$(dirname ${lib#/})"
        cp -L "$lib" "$dest/$(dirname ${lib#/})/" 2>/dev/null || true
        todo="$todo $lib"
        echo "$lib" >> /tmp/processed_$$.txt 2>/dev/null || true
      fi
    done
  done
  rm -f /tmp/processed_$$.txt
}

echo "Kütüphaneler paketleniyor..."
# Copy the main binary's dependencies
mkdir -p "${APP_DIR}/usr/lib"
for lib in $(ldd "${APP_DIR}/usr/bin/macros" 2>/dev/null | awk '/=> \//{print $3}' | sort -u); do
  cp -L "$lib" "${APP_DIR}/usr/lib/" 2>/dev/null || true
done

# Also bundle webkit dependencies recursively 
for lib in "${APP_DIR}/usr/lib/"*.so*; do
  for deplib in $(ldd "$lib" 2>/dev/null | awk '/=> \//{print $3}' | sort -u); do
    cp -L "$deplib" "${APP_DIR}/usr/lib/" 2>/dev/null || true
  done
done

# Set rpath
patchelf --set-rpath '$ORIGIN/:$ORIGIN/../lib' "${APP_DIR}/usr/bin/macros" 2>/dev/null || true

# Create AppImage
echo "AppImage paketleniyor..."
UPDINFO=0 ./appimagetool "${APP_DIR}"

# Rename
for f in "${APP_NAME}"*.AppImage; do
  [ -f "$f" ] && mv "$f" "${APP_NAME}.AppImage" && break
done 2>/dev/null || true
