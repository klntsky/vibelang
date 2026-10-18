#!/bin/sh
# Scaffolds a new Effeck app from a registry with each named package manager,
# then installs, builds, starts, and tests it, and checks the compiler version.
#
#   sh scripts/release/check-app.sh <registry-url> <version> npm bun
#
# HOME and the package-manager caches point into a temporary directory, so the
# check sees neither ~/.npmrc nor ~/.bun-create nor cached packages. KEEP=1
# keeps that directory for inspection.
set -eu
registry=$1
version=$2
shift 2
scratch=$(mktemp -d "${TMPDIR:-/tmp}/effeck-check.XXXXXX")
trap '[ -n "${KEEP:-}" ] && echo "check-app: kept $scratch" || rm -rf "$scratch"' EXIT

export HOME="$scratch/home"
mkdir -p "$HOME"
export npm_config_registry="$registry" NPM_CONFIG_REGISTRY="$registry" BUN_CONFIG_REGISTRY="$registry"
export npm_config_cache="$scratch/npm-cache" BUN_INSTALL_CACHE_DIR="$scratch/bun-cache"
# bun create runs bunx, which keeps create-effeck@latest under $TMPDIR across
# runs and reuses it even when the registry has a newer tarball, so an earlier
# rehearsal's template would be scaffolded. A scratch TMPDIR starts it clean.
# (https://github.com/oven-sh/bun/issues/12245; bunx re-resolves a bare name only after 24 hours.)
export TMPDIR="$scratch/tmp"
mkdir -p "$TMPDIR"
export npm_config_yes=true npm_config_fund=false npm_config_audit=false npm_config_update_notifier=false

case "$(uname -s)-$(uname -m)" in
  Darwin-arm64) platform=darwin-arm64 ;;
  Darwin-x86_64) platform=darwin-x64 ;;
  Linux-x86_64) platform=linux-x64 ;;
  Linux-aarch64) platform=linux-arm64 ;;
  *) echo "check-app: unsupported platform $(uname -s)-$(uname -m)" >&2; exit 1 ;;
esac

for manager in "$@"; do
  app="$scratch/$manager-app"
  step() { echo "check-app: $manager: $*"; }
  cd "$scratch"
  case $manager in
    npm) step "npm create effeck@$version"; npm create "effeck@$version" "$manager-app" ;;
    bun) step "bun create effeck"; bun create effeck "$manager-app" ;;
    *) echo "check-app: unknown package manager $manager" >&2; exit 1 ;;
  esac
  cd "$app"
  [ -d node_modules ] || { step "install"; "$manager" install; }

  step "installed packages"
  node --input-type=module - "$version" "$platform" <<'JS'
import { existsSync, readFileSync, realpathSync } from "node:fs"
import { createRequire } from "node:module"
const [version, platform] = process.argv.slice(2)
const manifest = (name) => JSON.parse(readFileSync(`node_modules/${name}/package.json`, "utf8"))
for (const name of ["effeck", "effeck-runtime", "effeck-vite", `effeck-${platform}`]) {
  if (manifest(name).version !== version) throw new Error(`${name} is ${manifest(name).version}, expected ${version}`)
}
if (existsSync("node_modules/effeck/native")) throw new Error("effeck ships a native/ directory")
if (!existsSync("node_modules/effeck/editor/effeck.vsix")) throw new Error("effeck lacks editor/effeck.vsix")
// Node resolves imports from a module's real path: the runtime and the app must load one effect.
const runtime = createRequire(realpathSync("node_modules/effeck-runtime/dist/index.js")).resolve("effect")
const project = createRequire(realpathSync("package.json")).resolve("effect")
if (runtime !== project) throw new Error(`the runtime loads ${runtime}, the app loads ${project}`)
console.log(`check-app: effeck, effeck-runtime, effeck-vite, effeck-${platform} at ${version}; one copy of effect`)
JS

  step "effeck --version"
  printed=$(node_modules/.bin/effeck --version)
  case $printed in
    "Effeck $version ("*) echo "check-app: $printed" ;;
    *) echo "check-app: effeck --version printed '$printed'" >&2; exit 1 ;;
  esac

  step "run build"
  "$manager" run build
  step "run start"
  "$manager" run start >"$scratch/$manager-start.txt" 2>&1 || { cat "$scratch/$manager-start.txt"; exit 1; }
  cat "$scratch/$manager-start.txt"
  /usr/bin/grep -Fq 'Hello, Effeck!' "$scratch/$manager-start.txt"
  /usr/bin/grep -Fq 'Recovered from EmptyName' "$scratch/$manager-start.txt"
  step "run test"
  "$manager" run test
  echo "check-app: $manager: PASS"
done
echo "check-app: PASS ($* on $platform, Effeck $version from $registry)"
