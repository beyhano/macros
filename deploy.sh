#!/usr/bin/env bash
set -euo pipefail

VERSION=$(python3 -c "import json; print(json.load(open('version.json'))['current']['version'])")

echo "🚀 macros v$VERSION dağıtımı başlıyor..."
echo ""

# 1. Commit & push
git add .
git commit -m "v$VERSION" --allow-empty
git push
echo ""

# 2. Build (linux + windows)
echo "🏗️  Linux build..."
wails3 task build >/dev/null 2>&1
echo "🏗️  Windows build..."
wails3 task windows:build >/dev/null 2>&1
echo ""

# 3. GitHub Release
echo "📤 GitHub Release oluşturuluyor..."
CHANGELOG=$(python3 -c "import json; h=json.load(open('version.json'))['history']; print(h[0]['changelog'] if h else 'Yeni sürüm')")
gh release create "v$VERSION" \
  --title "v$VERSION" \
  --notes "$CHANGELOG" \
  "./bin/macros#macros-linux-amd64" \
  "./bin/macros.exe#macros-windows-amd64.exe"

echo ""
echo "✅ v$VERSION yayınlandı!"
echo "   https://github.com/beyhano/macros/releases/tag/v$VERSION"
