#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")" && pwd)"
cd "$ROOT"

VERSION=$(python3 -c "import json; print(json.load(open('version.json'))['current']['version'])")

echo "🚀 macros v$VERSION dağıtımı başlıyor..."
echo ""

# 1. Commit & push
git add .
git commit -m "v$VERSION" --allow-empty
git push
echo ""

# 2. Build Linux binary
echo "🏗️  Linux binary..."
wails3 task build

# 3. Build Windows binary
echo "🏗️  Windows binary..."
wails3 task windows:build

# 4. Build Linux AppImage (custom build.sh with DISABLE_PLUGINS=1)
echo "🏗️  Linux AppImage..."
rm -rf "$ROOT/build/linux/appimage/build/macros.AppDir" "$ROOT/build/linux/appimage/build/squashfs-root"
rm -f "$ROOT/build/linux/appimage/build/macros.AppImage" "$ROOT/build/linux/appimage/build/macros-x86_64.AppImage"

cd "$ROOT/build/linux/appimage/build"
DISABLE_PLUGINS=1 \
NO_STRIP=1 \
APP_NAME=macros \
APP_BINARY="$ROOT/bin/macros" \
ICON_PATH="$ROOT/build/appicon.png" \
DESKTOP_FILE="$ROOT/build/linux/macros.desktop" \
bash "$ROOT/build/linux/appimage/build.sh" 2>&1 || true
cd "$ROOT"

# build.sh, appimagetool'un ürettiği macros-x86_64.AppImage'ı macros.AppImage'a
# yeniden adlandırır; kontrol bu isim üzerinden yapılmalı
APPIMAGE_OUT="$ROOT/build/linux/appimage/build/macros.AppImage"
if [ -f "$APPIMAGE_OUT" ]; then
  cp "$APPIMAGE_OUT" "$ROOT/bin/macros.AppImage"
  echo "   ✅ AppImage: bin/macros.AppImage"
else
  echo "   ⚠️  AppImage oluşturulamadı"
fi
echo ""

# 5. Build Windows NSIS installer (per-user scope so the integrated Wails
#    updater can replace the exe without admin rights)
echo "🏗️  Windows NSIS installer (per-user)..."
wails3 task windows:package INSTALL_SCOPE=user
if [ -f "bin/macros-amd64-installer.exe" ]; then
  cp "bin/macros-amd64-installer.exe" "$ROOT/bin/macros-installer.exe"
  echo "   ✅ Installer: bin/macros-installer.exe"
else
  echo "   ⚠️  NSIS installer oluşturulamadı"
fi
echo ""

# 6. GitHub Release
echo "📤 GitHub Release gönderiliyor..."
CHANGELOG=$(python3 -c "import json; h=json.load(open('version.json'))['history']; print(h[0]['changelog'] if h else 'Yeni sürüm')")

ASSETS=(
  "./bin/macros#macros-linux-amd64"
  "./bin/macros.exe#macros-windows-amd64.exe"
)

# SHA256SUMS sidecar — filenames MUST match the asset basenames exactly.
HASH_FILES="macros macros.exe"
[ -f bin/macros-installer.exe ] && HASH_FILES="$HASH_FILES macros-installer.exe"
if command -v sha256sum >/dev/null 2>&1; then
  (cd "$ROOT/bin" && sha256sum $HASH_FILES > SHA256SUMS)
else
  (cd "$ROOT/bin" && shasum -a 256 $HASH_FILES > SHA256SUMS)
fi
ASSETS+=("./bin/SHA256SUMS#SHA256SUMS")

[ -f bin/macros-installer.exe ] && ASSETS+=("./bin/macros-installer.exe#macros-installer.exe")

[ -f bin/macros.AppImage ] && ASSETS+=("./bin/macros.AppImage#macros-linux-amd64.AppImage")

if gh release view "v$VERSION" >/dev/null 2>&1; then
  gh release upload "v$VERSION" "${ASSETS[@]}" --clobber
else
  gh release create "v$VERSION" \
    --title "v$VERSION" \
    --notes "$CHANGELOG" \
    "${ASSETS[@]}"
fi

echo ""
echo "✅ v$VERSION yayınlandı!"
echo "   https://github.com/beyhano/macros/releases/tag/v$VERSION"
