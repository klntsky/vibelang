#!/bin/sh
# Rehearses the npm release: publishes dist-npm/tarballs/ to a throwaway local
# Verdaccio registry, then scaffolds, builds, starts, and tests an app with npm
# and with Bun against it. Effeck's own packages come only from the local
# registry; everything else is proxied from registry.npmjs.org.
#
#   sh scripts/release/rehearse.sh
#
# Run scripts/release/build-binaries.sh and make-packages.mjs first.
# EFFECK_SKIP=win32 leaves out every package whose name contains win32, the way
# publish.sh does, to rehearse a release that ships without those packages.
# Both scripts repack effeck to omit those optional dependencies too.
set -eu
root=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
tarballs="$root/dist-npm/tarballs"
verdaccio=verdaccio@6.10.4
[ -f "$tarballs/manifest.json" ] || { echo "rehearse: run scripts/release/make-packages.mjs first" >&2; exit 1; }
version=$(node -p 'require(process.argv[1]).version' "$tarballs/manifest.json")
node "$root/scripts/release/repack.mjs" "$tarballs"
scratch=$(mktemp -d "${TMPDIR:-/tmp}/effeck-rehearse.XXXXXX")
pid=""
# npx runs Verdaccio as a descendant process, and on Linux Verdaccio renames
# its process title, so neither killing npx nor matching the config path stops
# it. killtree stops npx and every process under it.
# Cleanup must not change the exit status: kill fails once a process is gone.
killtree() {
  for child in $(pgrep -P "$1" 2>/dev/null); do killtree "$child"; done
  kill "$1" 2>/dev/null || true
}
trap '[ -z "$pid" ] || killtree "$pid"; pkill -f "$scratch/config.yaml" 2>/dev/null || true; rm -rf "$scratch"' EXIT HUP INT TERM
port=$(node -e 'const s=require("net").createServer().listen(0,"127.0.0.1",()=>{console.log(s.address().port);s.close()})')
registry="http://127.0.0.1:$port/"

# The platform tarballs are about 10 MB, Verdaccio's default request limit.
cat >"$scratch/config.yaml" <<EOF
storage: $scratch/storage
max_body_size: 200mb
auth:
  htpasswd:
    file: $scratch/htpasswd
uplinks:
  npmjs:
    url: https://registry.npmjs.org/
packages:
  'effeck':
    access: \$all
    publish: \$authenticated
  'effeck-*':
    access: \$all
    publish: \$authenticated
  'create-effeck':
    access: \$all
    publish: \$authenticated
  '**':
    access: \$all
    proxy: npmjs
log: { type: stdout, format: pretty, level: warn }
EOF

echo "rehearse: start $verdaccio at $registry"
npx --yes "$verdaccio" --config "$scratch/config.yaml" --listen "127.0.0.1:$port" >"$scratch/verdaccio.log" 2>&1 &
pid=$!
tries=0
until curl -fsS "${registry}-/ping" >/dev/null 2>&1; do
  tries=$((tries + 1))
  [ "$tries" -lt 120 ] || { cat "$scratch/verdaccio.log" >&2; echo "rehearse: Verdaccio did not start" >&2; exit 1; }
  sleep 1
done

token=$(curl -fsS -X PUT -H 'content-type: application/json' \
  -d '{"name":"rehearsal","password":"rehearsal-only"}' \
  "${registry}-/user/org.couchdb.user:rehearsal" | node -p 'JSON.parse(require("fs").readFileSync(0, "utf8")).token')
printf '//127.0.0.1:%s/:_authToken=%s\n' "$port" "$token" >"$scratch/publish.npmrc"

node -e 'for (const p of require(process.argv[1]).packages) console.log(p.name, p.file)' "$tarballs/manifest.json" |
  while read -r name file; do
    skip=""
    for pattern in $(printf '%s' "${EFFECK_SKIP:-}" | tr ',' ' '); do
      case $name in *"$pattern"*) skip=1 ;; esac
    done
    [ -z "$skip" ] || { echo "rehearse: left out $name (EFFECK_SKIP)"; continue; }
    npm publish "$tarballs/$file" --registry "$registry" --userconfig "$scratch/publish.npmrc" --access public --ignore-scripts >/dev/null 2>"$scratch/publish.err" ||
      { cat "$scratch/publish.err" >&2; echo "rehearse: publishing $name failed" >&2; exit 1; }
    echo "rehearse: published $name@$version"
  done

sh "$root/scripts/release/check-app.sh" "$registry" "$version" npm bun
echo "rehearse: PASS"
