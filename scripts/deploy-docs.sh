#!/bin/sh
# Publishes the docs on Cloudflare: the user guide at / and the specification
# at /spec/, served by a static-assets Worker on effeck.dev. A second Worker
# (docs/redirect/) sends www.effeck.dev and the other effeck.* domains there.
#
#   sh scripts/deploy-docs.sh             # build, check, and deploy
#   sh scripts/deploy-docs.sh --dry-run   # build and check into .cache/docs-site only
#
# Deploying needs CLOUDFLARE_API_TOKEN with Workers Scripts edit access.
# CLOUDFLARE_ACCOUNT_ID selects the account (default: Will's personal account).
# docs/wrangler.jsonc names the Worker, effeck. docs/tools/prepare-site.ts
# refuses to publish local paths, host names, or credentials.
set -eu
root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
site="$root/.cache/docs-site"
dry_run=false
case "${1:-}" in
	--dry-run) dry_run=true ;;
	"") ;;
	*) echo "usage: sh scripts/deploy-docs.sh [--dry-run]" >&2; exit 2 ;;
esac

# The spec build runs the kernel's law suite first, which needs these installs.
for dir in docs/tools experiments packages/runtime; do
	[ -d "$root/$dir/node_modules" ] || (cd "$root/$dir" && bun install --silent)
done
(cd "$root/docs/tools" && bun render.ts)
bun "$root/docs/tools/render-user.ts"

rm -rf "$site"
mkdir -p "$site/spec"
cp -R "$root/docs/user/site/." "$site/"
cp "$root/docs/design/effeck-design.html" "$site/spec/index.html"
bun "$root/docs/tools/prepare-site.ts" "$site"

if $dry_run; then
	echo "deploy-docs: built $site; nothing deployed"
	exit 0
fi
: "${CLOUDFLARE_API_TOKEN:?deploy-docs: set CLOUDFLARE_API_TOKEN to deploy}"
export CLOUDFLARE_ACCOUNT_ID="${CLOUDFLARE_ACCOUNT_ID:-dd3525a4132493566aeb38de533c8827}"
cd "$root"
bunx wrangler@4.151.1 deploy --config docs/wrangler.jsonc
bunx wrangler@4.151.1 deploy --config docs/redirect/wrangler.jsonc
