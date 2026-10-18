#!/bin/sh
# Publishes the docs on Cloudflare: one site with the user guide at / and the
# specification's chapters under /spec/, served by a static-assets Worker on
# effeck.dev. A second Worker (docs/redirect/) sends www.effeck.dev and the
# other effeck.* domains there.
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

# The site build runs the kernel's law suite first, which needs these installs.
for dir in docs/tools experiments packages/runtime; do
	[ -d "$root/$dir/node_modules" ] || (cd "$root/$dir" && bun install --silent)
done
bun "$root/docs/tools/render-user.ts"

# Until 0.0.2 ships, every docs example must compile with the release that
# `npm create effeck@latest` installs: the 0.0.1 native compiler and runtime
# from the release tarballs, not this checkout's build.
# EFFECK_SKIP_RELEASE_SNIPPETS=1 skips the check, and needs
# EFFECK_SKIP_RELEASE_SNIPPETS_REASON, which is printed.
release=${EFFECK_RELEASE_VERSION:-0.0.1}
tarballs=${EFFECK_RELEASE_TARBALLS:-$root/dist-npm/tarballs}
if [ "${EFFECK_SKIP_RELEASE_SNIPPETS:-}" = 1 ]; then
	echo "deploy-docs: skipping the $release release snippet check: ${EFFECK_SKIP_RELEASE_SNIPPETS_REASON:?deploy-docs: set EFFECK_SKIP_RELEASE_SNIPPETS_REASON to say why}"
else
	case "$(uname -s)-$(uname -m)" in
		Darwin-arm64) platform=darwin-arm64 ;;
		Darwin-x86_64) platform=darwin-x64 ;;
		Linux-aarch64 | Linux-arm64) platform=linux-arm64 ;;
		Linux-x86_64) platform=linux-x64 ;;
		*) echo "deploy-docs: no $release release compiler for $(uname -s)-$(uname -m)" >&2; exit 1 ;;
	esac
	release_dir=$(mktemp -d "${TMPDIR:-/tmp}/effeck-release.XXXXXX")
	trap 'rm -rf "$release_dir"' EXIT
	for name in "effeck-$platform" effeck-runtime; do
		mkdir -p "$release_dir/$name"
		tar xzf "$tarballs/$name-$release.tgz" -C "$release_dir/$name" 2>/dev/null ||
			{ echo "deploy-docs: missing $tarballs/$name-$release.tgz; set EFFECK_RELEASE_TARBALLS" >&2; exit 1; }
	done
	# As an npm install puts effect beside effeck-runtime, give the extracted runtime
	# the repository's pinned effect, so its declarations resolve their imports.
	mkdir -p "$release_dir/effeck-runtime/package/node_modules"
	ln -s "$(cd "$root/packages/runtime/node_modules/effect" && pwd -P)" "$release_dir/effeck-runtime/package/node_modules/effect"
	echo "deploy-docs: compiling every docs example with effeck $release ($platform) and effeck-runtime $release"
	EFFECK_BINARY="$release_dir/effeck-$platform/package/bin/effeck" EFFECK_RUNTIME_DIR="$release_dir/effeck-runtime/package" \
		bun "$root/docs/tools/check-snippets.ts" > "$release_dir/snippets.log" ||
		{ tail -40 "$release_dir/snippets.log" >&2; echo "deploy-docs: a docs example does not compile with the $release release" >&2; exit 1; }
	tail -1 "$release_dir/snippets.log"
fi

rm -rf "$site"
mkdir -p "$site"
cp -R "$root/docs/user/site/." "$site/"
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
