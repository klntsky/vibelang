#!/bin/sh
# Publishes Effeck's ten npm packages from dist-npm/tarballs/ in order: the six
# platform compilers, effeck, effeck-runtime, effeck-vite, and create-effeck.
# Run it yourself in a terminal, after npm login:
#
#   sh scripts/release/publish.sh            # asks for a one-time password once
#   sh scripts/release/publish.sh --dry-run  # checks everything, publishes nothing
#   NPM_OTP=123456 sh scripts/release/publish.sh
#
# A package already published at this version is skipped, so rerunning after a
# failure resumes where it stopped. If npm rejects the one-time password, the
# script asks for a new one and retries that package.
set -eu
root=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
tarballs="$root/dist-npm/tarballs"
# EFFECK_REGISTRY points the script at another registry, such as a local Verdaccio.
registry=${EFFECK_REGISTRY:-https://registry.npmjs.org/}
dry=""
[ "${1:-}" = "--dry-run" ] && dry=--dry-run

[ -f "$tarballs/manifest.json" ] || {
  echo "publish: no dist-npm/tarballs/manifest.json. Run sh scripts/release/build-binaries.sh and node scripts/release/make-packages.mjs first." >&2
  exit 1
}
version=$(node -p 'require(process.argv[1]).version' "$tarballs/manifest.json")

if user=$(npm whoami --registry "$registry" 2>/dev/null); then
  echo "publish: logged in to npm as $user; publishing Effeck $version"
elif [ -n "$dry" ]; then
  echo "publish: not logged in to npm; the dry run continues"
else
  echo "publish: you are not logged in to npm. Run npm login, then rerun this script." >&2
  exit 1
fi

todo=""
for item in $(node -e 'for (const p of require(process.argv[1]).packages) console.log(`${p.name}=${p.file}`)' "$tarballs/manifest.json"); do
  name=${item%%=*}
  file=${item#*=}
  packed=$(tar -xOzf "$tarballs/$file" package/package.json | node -p 'const p = JSON.parse(require("fs").readFileSync(0, "utf8")); `${p.name}@${p.version}`')
  [ "$packed" = "$name@$version" ] || { echo "publish: $file contains $packed, expected $name@$version" >&2; exit 1; }
  if npm view "$name@$version" version --registry "$registry" >/dev/null 2>&1; then
    echo "publish: $name@$version is already on npm; skipping"
  else
    todo="$todo $item"
  fi
done
[ -n "$todo" ] || { echo "publish: all ten packages are already on npm at $version."; exit 0; }

otp=${NPM_OTP:-}
if [ -z "$dry" ] && [ -z "$otp" ]; then
  printf 'npm one-time password (press Enter if your account does not use one): '
  read -r otp
fi

for item in $todo; do
  name=${item%%=*}
  file=${item#*=}
  while :; do
    set -- npm publish "$tarballs/$file" --registry "$registry" --access public
    [ -z "$otp" ] || set -- "$@" --otp "$otp"
    [ -z "$dry" ] || set -- "$@" --dry-run
    if "$@"; then
      if [ -n "$dry" ]; then echo "publish: $name@$version checked (dry run)"; else echo "publish: $name@$version published"; fi
      break
    fi
    [ -z "$dry" ] || exit 1
    printf 'publish: %s failed. Enter a new one-time password to retry, or press Enter to stop: ' "$name"
    read -r otp
    [ -n "$otp" ] || { echo "publish: stopped. Rerun this script to resume." >&2; exit 1; }
  done
done

[ -n "$dry" ] && { echo "publish: dry run passed for $version"; exit 0; }
echo "publish: published Effeck $version. Check it from the public registry with:"
echo "  sh scripts/release/verify-published.sh $version"
