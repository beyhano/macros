# AppImage Build

Builds the macros Wails v3 desktop app as a Linux x86_64 AppImage via `scripts/build-appimage.sh` (native binary + packaging + verification, end to end).

Pipeline: `wails3 task build` produces `bin/macros`, then `build/linux/appimage/build.sh` (run with its working directory set to `build/linux/appimage/build`) assembles the AppDir with `linuxdeploy` / `appimagetool` and `ldd` bundling, and the result is copied to the final destination.

### What the script does, step by step

1. Resolves the repo root by walking up from `scripts/` until `version.json` and `build/linux/appimage/build.sh` are both found (fails with exit 1 otherwise).
2. Parses flags; `--help` and `--version` print and exit 0. `--output` is resolved before any directory change, so relative paths stay relative to the invoking shell's directory.
3. `--clean` removes the outputs listed under Flags, keeps the toolchain cache, prints `clean done`, and exits 0 without building.
4. Warns (but continues) on non-Linux systems or non-`x86_64` architectures, since `linuxdeploy` is `x86_64`-only.
5. Checks dependencies (`go`, `gcc`/`clang`, `wget`, `file`, `ldd`, `timeout`, `python3`, plus `wails3` unless `--skip-build`); any miss exits 2.
6. Runs `wails3 task build` (or reuses `bin/macros` with `--skip-build`), then requires `bin/macros` to exist and be an ELF binary; failures exit 3.
7. Ensures `mksquashfs` (system or rootless cache), runs the packager from the workdir, copies the produced AppImage to `--output` with the executable bit, verifies it with `file`, runs the smoke test, and prints the `OK:` summary.

## Prerequisites

- Linux x86_64, Go 1.25+, `gcc` or `clang`.
- Tools: `wails3`, `wget`, `file`, `ldd` (glibc), `timeout`, `python3`, plus `tar` and `zstd` only if the rootless `mksquashfs` fallback is needed.
- Network access for `linuxdeploy` / `appimagetool` (and for the `squashfs-tools` package if the fallback triggers).
- `sudo` is never needed: the script never calls `sudo`, and the rootless `mksquashfs` fallback is automatic (see Troubleshooting).

## Quick Start

```bash
scripts/build-appimage.sh                    # full build -> bin/macros.AppImage
scripts/build-appimage.sh --skip-build       # reuse existing bin/macros, packaging only
scripts/build-appimage.sh --clean            # remove AppDir/build outputs and bin/*.AppImage, then exit
```

More examples (from `--help`):

```bash
scripts/build-appimage.sh --skip-build
scripts/build-appimage.sh --output ./dist/macros.AppImage
scripts/build-appimage.sh --version          # prints e.g. "macros appimage builder 0.0.10"
```

## Flags and Exit Codes

| Flag | Meaning |
|------|---------|
| `--output PATH` (or `--output=PATH`) | AppImage destination (default: `bin/macros.AppImage`); relative paths resolve against the current directory |
| `--skip-build` | Reuse `bin/macros` instead of running `wails3 task build` |
| `--clean` | Remove `build/linux/appimage/build` outputs and `bin/*.AppImage`, then exit (keeps the cached rootless `mksquashfs` toolchain) |
| `--no-smoke` | Skip the post-build smoke test (the smoke test runs by default) |
| `--trace` | Enable `set -x` debug tracing |
| `-h, --help` | Show help and exit |
| `--version` | Print the builder version (read from `version.json`) and exit |

Notes:

- `--output` requires a value; `--output=PATH` is accepted as well. Unknown flags exit 1.
- `--clean` deletes `build/linux/appimage/build/macros.AppDir`, `build/linux/appimage/build/squashfs-root`, `build/linux/appimage/build/appimagetool`, any `build/linux/appimage/build/*.AppImage` and `linuxdeploy-*` files, plus `bin/*.AppImage`. It exits 0 without building.
- `--skip-build` also skips the `wails3` dependency check; every other check still runs.

Exit codes: `0` ok, `1` general error (bad usage, missing input file, copy failure), `2` missing dependency, `3` native build failure (includes missing or non-ELF `bin/macros`), `4` packaging failure (includes `mksquashfs` download/extract failure), `5` verification failure.

## Outputs

- The AppImage lands at `bin/macros.AppImage` by default, or at the `--output` path. The packager first produces it under `build/linux/appimage/build/` (`macros.AppImage`, falling back to the first match of `macros-*.AppImage` or `*.AppImage` there), then copies it to the destination and marks it executable.
- On success the script prints `OK: <path> (<size>)`, `Run it: <path>`, and a note to attach the file to a GitHub release.
- Run it:

```bash
./bin/macros.AppImage
```

- Verification: `file` must report `ELF`, `AppImage`, `ISO 9660`, or `squashfs` for the output, otherwise the build fails with exit 5.
- Smoke test (runs by default): executes `timeout 30 -- <AppImage> --appimage-extract-and-run --help` and reports success on exit 0. A non-zero exit only logs a warning, since GUI apps often ignore `--help`; the build still succeeds. Use `--no-smoke` to skip it entirely.

## Troubleshooting

- `mksquashfs` missing: a system copy is preferred; otherwise the script downloads the Arch `squashfs-tools` package and extracts only `usr/bin/mksquashfs` into the cache at `${XDG_CACHE_HOME:-~/.cache}/macros-appimage/sqfs` (i.e. `~/.cache/macros-appimage/sqfs` by default), prepends it to `PATH`, and continues. No root required.
- No network: `linuxdeploy` / `appimagetool` downloads (and the fallback package above) fail; the build exits non-zero (packaging failure, code 4). Restore network access and retry.
- `wails3` missing: exit 2 with an install hint. Either install `wails3` (https://v3.wails.io) or pass `--skip-build` with an existing `bin/macros` binary.
- `binary missing` / `not an ELF binary`: the native step did not produce `bin/macros`. Run without `--skip-build` so `wails3 task build` runs, or fix the native build first (exit 3).
- `frontend/dist looks missing`: warning only; the binary may embed a stale frontend.
- `--version` (currently `0.0.10`) is read from `version.json` (`current.version`) via `python3` and is read-only; the script never writes to it.
- `bin/` is gitignored, so `bin/macros.AppImage` is never committed. `scripts/` is new/untracked, so commit `scripts/build-appimage.sh` itself for others to build.
- Pipeline inputs the script verifies: `bin/macros` (must be an ELF binary), `frontend/dist/index.html` (warns only if stale/missing), `build/appicon.png`, `build/linux/macros.desktop`, `build/linux/appimage/build.sh`.

## Release Note

`deploy.sh` picks up `bin/macros.AppImage` automatically: after building, it attaches the file to the GitHub release for the current `version.json` version as `macros-linux-amd64.AppImage` (via `gh release create` / `gh release upload --clobber`), alongside the Windows and raw-binary assets.
