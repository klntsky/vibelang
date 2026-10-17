#!/bin/sh
# Cross-compiles the native Effeck compiler for the six npm platforms into
# dist-npm/bin/<platform>/effeck (effeck.exe on Windows).
#
#   ES_REMOTE_HOST=user@host ES_REMOTE_KEY=~/.ssh/key sh scripts/release/build-binaries.sh
#   sh scripts/release/build-binaries.sh --local
#
# With ES_REMOTE_HOST set, the six Go builds run on that host through
# scripts/remote.sh and the binaries are copied back, so this machine needs
# neither the disk nor the CPU for them. --local builds here instead. Either
# way, the binaries this machine can run must print the expected version.
set -eu
root=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
out="$root/dist-npm/bin"
platforms="darwin-arm64 darwin-x64 linux-arm64 linux-x64 win32-arm64 win32-x64"
version=$(node -p 'require(process.argv[1]).version' "$root/packages/compiler/package.json")

check() {
  # check <platform> runs one binary's --version.
  platform=$1
  printed=$("$out/$platform/effeck" --version)
  case $printed in
    "Effeck $version ("*) echo "build-binaries: $platform prints: $printed" ;;
    *) echo "build-binaries: $platform printed '$printed', expected Effeck $version" >&2; exit 1 ;;
  esac
}

if [ "${1:-}" != "--local" ] && [ -n "${ES_REMOTE_HOST:-}" ]; then
  key=${ES_REMOTE_KEY:-$HOME/.ssh/id_ed25519}
  (cd "$root" && sh scripts/remote.sh sh scripts/release/build-binaries.sh --local)
  # remote.sh keeps the workspace under ~/es-remote/<workspace directory name>.
  mkdir -p "$out"
  rsync -a --delete -e "ssh -i $key -o IdentitiesOnly=yes -o BatchMode=yes" \
    "$ES_REMOTE_HOST:es-remote/$(basename "$root")/dist-npm/bin/" "$out/"
else
  command -v go >/dev/null 2>&1 || { echo "build-binaries: Go is required." >&2; exit 1; }
  rm -rf "$out"
  mkdir -p "$out"
  pids=""
  for platform in $platforms; do
    os=${platform%-*}
    cpu=${platform#*-}
    exe=effeck
    goos=$os
    [ "$os" = win32 ] && { goos=windows; exe=effeck.exe; }
    goarch=$cpu
    [ "$cpu" = x64 ] && goarch=amd64
    mkdir -p "$out/$platform"
    echo "build-binaries: GOOS=$goos GOARCH=$goarch go build ./cmd/tsc -> dist-npm/bin/$platform/$exe"
    (cd "$root/typescript/tsc" && CGO_ENABLED=0 GOOS=$goos GOARCH=$goarch \
      go build -trimpath -ldflags="-s -w" -o "$out/$platform/$exe" ./cmd/tsc) >"$out/$platform.log" 2>&1 &
    pids="$pids $!"
  done
  failed=0
  for pid in $pids; do wait "$pid" || failed=1; done
  if [ "$failed" -ne 0 ]; then
    for platform in $platforms; do cat "$out/$platform.log" >&2; done
    echo "build-binaries: a build failed" >&2
    exit 1
  fi
  rm -f "$out"/*.log
fi

for platform in $platforms; do
  exe=effeck
  case $platform in win32-*) exe=effeck.exe ;; esac
  [ -f "$out/$platform/$exe" ] || { echo "build-binaries: missing dist-npm/bin/$platform/$exe" >&2; exit 1; }
  printf 'build-binaries: %-13s %s bytes\n' "$platform" "$(wc -c <"$out/$platform/$exe" | tr -d ' ')"
done

case "$(uname -s)-$(uname -m)" in
  Darwin-arm64)
    check darwin-arm64 ;;
  Darwin-x86_64) check darwin-x64 ;;
  Linux-x86_64) check linux-x64 ;;
  Linux-aarch64) check linux-arm64 ;;
esac
