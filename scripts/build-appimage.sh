#!/usr/bin/env bash
# build-appimage.sh — end-to-end AppImage build for the macros desktop app.
# Pipeline: wails3 task build -> bin/macros -> build/linux/appimage/build.sh
# (AppDir + linuxdeploy/appimagetool + ldd bundling) -> bin/macros.AppImage.
# Needs: Linux x86_64, go 1.25+, gcc/clang, wails3, wget, file, ldd,
# timeout, python3, network (linuxdeploy/appimagetool, maybe mksquashfs).
# Exit codes: 0 ok, 1 general, 2 missing dep, 3 native build fail,
# 4 packaging fail, 5 verification fail.

set -Eeuo pipefail
shopt -s inherit_errexit 2>/dev/null || true

readonly APP_NAME="macros"
readonly SQFS_URL="https://archlinux.org/packages/core/x86_64/squashfs-tools/download/"
SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)"

# Repo root: walk up from the script until the project markers are found.
resolve_root() {
  local dir="${SCRIPT_DIR}"
  while [[ "${dir}" != "/" ]]; do
    [[ -f "${dir}/version.json" && -f "${dir}/build/linux/appimage/build.sh" ]] \
      && { printf '%s' "${dir}"; return 0; }
    dir="$(dirname -- "${dir}")"
  done
  return 1
}
ROOT="$(resolve_root)" || { printf '[?] ERROR: repo root not found\n' >&2; exit 1; }
readonly ROOT

log() { printf '[%s] %s: %s\n' "$(date '+%H:%M:%S')" "$1" "${*:2}" >&2; }
log_info() { log "INFO" "$@"; }
log_warn() { log "WARN" "$@"; }
log_error() { log "ERROR" "$@"; }

SKIP_BUILD=0; DO_CLEAN=0; NO_SMOKE=0; TRACE=0; OUTPUT=""
read_version() {
  python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["current"]["version"])' "${ROOT}/version.json" 2>/dev/null || printf 'unknown'
}
usage() {
  cat <<'EOF'
Usage: build-appimage.sh [OPTIONS]
Build the macros AppImage end-to-end (native binary + packaging + verify).
  --output PATH   AppImage destination (default: bin/macros.AppImage)
  --skip-build    Reuse bin/macros instead of running wails3 task build
  --clean         Remove AppDir/build outputs and bin/*.AppImage, then exit
                  (keeps the cached rootless mksquashfs toolchain)
  --no-smoke      Skip the post-build smoke test (smoke test runs by default)
  --trace         Enable 'set -x' debug tracing
  -h, --help      Show this help and exit
  --version       Print builder version (from version.json) and exit
Examples:
  scripts/build-appimage.sh --skip-build
  scripts/build-appimage.sh --output ./dist/macros.AppImage
Exit codes: 0 ok, 1 general, 2 missing dep, 3 native build fail,
  4 packaging fail, 5 verification fail.
Prerequisites: Linux x86_64, go 1.25+, gcc or clang, wails3, wget, file,
  ldd (glibc), timeout, python3, network access for linuxdeploy/appimagetool.
EOF
}
while [[ $# -gt 0 ]]; do
  case "$1" in
    -h|--help) usage; exit 0 ;;
    --version) printf 'macros appimage builder %s\n' "$(read_version)"; exit 0 ;;
    --skip-build) SKIP_BUILD=1 ;;
    --clean) DO_CLEAN=1 ;;
    --no-smoke) NO_SMOKE=1 ;;
    --trace) TRACE=1 ;;
    --output)
      [[ $# -ge 2 ]] || { log_error "--output needs a value"; exit 1; }
      OUTPUT="$2"; shift ;;
    --output=*)
      [[ -n "${1#*=}" ]] || { log_error "--output needs a value"; exit 1; }
      OUTPUT="${1#*=}" ;;
    --) shift; break ;;
    -*) log_error "unknown option: $1"; usage >&2; exit 1 ;;
    *) log_error "unexpected argument: $1"; usage >&2; exit 1 ;;
  esac
  shift
done
[[ "${TRACE}" -eq 1 ]] && set -x
log_info "repo root: ${ROOT}"

# Resolve --output before any cd; relative paths stay relative to the cwd.
if [[ -n "${OUTPUT}" ]]; then
  [[ "${OUTPUT}" == /* ]] || OUTPUT="${PWD}/${OUTPUT}"
else
  OUTPUT="${ROOT}/bin/${APP_NAME}.AppImage"
fi
readonly OUTPUT
readonly WORKDIR="${ROOT}/build/linux/appimage/build"
readonly APP_BINARY="${ROOT}/bin/macros"
readonly ICON_PATH="${ROOT}/build/appicon.png"
readonly DESKTOP_FILE="${ROOT}/build/linux/macros.desktop"
readonly PACKAGER="${ROOT}/build/linux/appimage/build.sh"

if [[ "${DO_CLEAN}" -eq 1 ]]; then
  log_info "cleaning AppImage build outputs (toolchain cache kept)"
  if [[ -d "${WORKDIR}" ]]; then
    rm -rf -- "${WORKDIR}/${APP_NAME}.AppDir" "${WORKDIR}/squashfs-root" "${WORKDIR}/appimagetool" || exit 1
    shopt -s nullglob
    rm -f -- "${WORKDIR}/"*.AppImage "${WORKDIR}/linuxdeploy-"* || exit 1
    shopt -u nullglob
  fi
  rm -f -- "${ROOT}/bin/"*.AppImage || exit 1
  log_info "clean done"
  exit 0
fi

[[ "$(uname -s)" == "Linux" ]] || log_warn "non-Linux OS ($(uname -s)); AppImage targets Linux, continuing"
[[ "$(uname -m)" == "x86_64" ]] || log_warn "non-x86_64 arch ($(uname -m)); linuxdeploy is x86_64-only, continuing"

require_cmd() { # $1 = command, $2 = install hint
  command -v "$1" >/dev/null 2>&1 || { log_error "missing dependency: $1 ($2)"; return 2; }
}
require_cmd go "https://go.dev/dl/ (need go 1.25+)" || exit 2
if ! command -v gcc >/dev/null 2>&1 && ! command -v clang >/dev/null 2>&1; then
  log_error "missing dependency: gcc or clang (apt install build-essential)"; exit 2
fi
require_cmd wget "apt install wget" || exit 2
require_cmd file "apt install file" || exit 2
require_cmd ldd "glibc-bin (apt install libc-bin)" || exit 2
require_cmd timeout "apt install coreutils" || exit 2
require_cmd python3 "apt install python3" || exit 2
if [[ "${SKIP_BUILD}" -eq 0 ]]; then
  command -v wails3 >/dev/null 2>&1 \
    || { log_error "wails3 not found; install it (https://v3.wails.io) or use --skip-build"; exit 2; }
fi

TMP_DIR="$(mktemp -d)"
cleanup() { [[ -n "${TMP_DIR:-}" && -d "${TMP_DIR}" ]] && rm -rf -- "${TMP_DIR}" || true; }
trap cleanup EXIT

# mksquashfs: system copy preferred; otherwise a rootless cached copy
# (Arch package, only usr/bin/mksquashfs extracted, PATH prepended).
ensure_mksquashfs() {
  if command -v mksquashfs >/dev/null 2>&1; then
    log_info "mksquashfs: $(mksquashfs -version 2>&1 | head -n 1)"; return 0
  fi
  log_warn "mksquashfs missing; installing rootless copy"
  require_cmd tar "apt install tar" || return 2
  require_cmd zstd "apt install zstd" || return 2
  local cache="${XDG_CACHE_HOME:-${HOME:-/tmp}/.cache}/macros-appimage/sqfs"
  mkdir -p -- "${cache}" || return 1
  if [[ ! -x "${cache}/usr/bin/mksquashfs" ]]; then
    wget -q -4 -O "${TMP_DIR}/squashfs-tools.pkg.tar.zst" "${SQFS_URL}" \
      || { log_error "failed to download squashfs-tools package"; return 4; }
    tar -I zstd -xf "${TMP_DIR}/squashfs-tools.pkg.tar.zst" -C "${cache}" usr/bin/mksquashfs \
      || { log_error "failed to extract mksquashfs from package"; return 4; }
  fi
  export PATH="${cache}/usr/bin:${PATH}"
  command -v mksquashfs >/dev/null 2>&1 || { log_error "rootless mksquashfs install failed"; return 4; }
  log_info "mksquashfs (rootless): $(mksquashfs -version 2>&1 | head -n 1)"
}

if [[ "${SKIP_BUILD}" -eq 1 ]]; then
  log_info "--skip-build: reusing existing binary"
else
  log_info "running native build: wails3 task build"
  ( cd -- "${ROOT}" && wails3 task build ) \
    || { log_error "wails3 task build failed; fix the native build first"; exit 3; }
fi
[[ -f "${APP_BINARY}" ]] || { log_error "binary missing: ${APP_BINARY}"; exit 3; }
file -- "${APP_BINARY}" | grep -q "ELF" || { log_error "not an ELF binary: ${APP_BINARY}"; exit 3; }
log_info "native binary OK: ${APP_BINARY}"
[[ -f "${ROOT}/frontend/dist/index.html" ]] || log_warn "frontend/dist looks missing; binary may embed a stale frontend"
for f in "${ICON_PATH}" "${DESKTOP_FILE}" "${PACKAGER}"; do
  [[ -f "${f}" ]] || { log_error "required input missing: ${f}"; exit 1; }
done

mkdir -p -- "${WORKDIR}" || exit 4
ensure_mksquashfs || { rc=$?; [[ "${rc}" -eq 2 ]] && exit 2; exit 4; }

# build.sh assembles AppDir relative to cwd, so run it from the workdir.
log_info "running packager (cwd=${WORKDIR})"
( cd -- "${WORKDIR}" \
  && APP_NAME="${APP_NAME}" APP_BINARY="${APP_BINARY}" \
     ICON_PATH="${ICON_PATH}" DESKTOP_FILE="${DESKTOP_FILE}" \
     bash -- "${PACKAGER}" ) || { log_error "build.sh packaging failed"; exit 4; }

# build.sh renames the result to <name>.AppImage; fall back to any match.
PRODUCED=""
if [[ -f "${WORKDIR}/${APP_NAME}.AppImage" ]]; then
  PRODUCED="${WORKDIR}/${APP_NAME}.AppImage"
else
  shopt -s nullglob
  for f in "${WORKDIR}/${APP_NAME}"-*.AppImage "${WORKDIR}/"*.AppImage; do PRODUCED="${f}"; break; done
  shopt -u nullglob
fi
[[ -n "${PRODUCED}" ]] || { log_error "no *.AppImage produced in ${WORKDIR}"; exit 4; }
log_info "produced: ${PRODUCED}"

mkdir -p -- "$(dirname -- "${OUTPUT}")" || exit 1
cp -- "${PRODUCED}" "${OUTPUT}" || { log_error "copy to ${OUTPUT} failed"; exit 4; }
chmod +x -- "${OUTPUT}" || exit 4

file -- "${OUTPUT}" | grep -qiE "ELF|AppImage|ISO 9660|squashfs" \
  || { log_error "verification failed: ${OUTPUT} does not look like an AppImage"; exit 5; }
log_info "file check OK: $(file -b -- "${OUTPUT}" | cut -c1-80)"

if [[ "${NO_SMOKE}" -eq 1 ]]; then
  log_info "smoke test skipped (--no-smoke)"
elif timeout 30 -- "${OUTPUT}" --appimage-extract-and-run --help >/dev/null 2>&1; then
  log_info "smoke test passed (exit 0)"
else
  log_warn "smoke test exited non-zero; GUI apps often ignore --help, ignoring"
fi

SIZE="$(du -h -- "${OUTPUT}" | cut -f1)"
printf 'OK: %s (%s)\n' "${OUTPUT}" "${SIZE}"
printf 'Run it: %s\n' "${OUTPUT}"
printf 'Note: attach %s to a GitHub release for distribution.\n' "$(basename -- "${OUTPUT}")"
