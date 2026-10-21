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
# The docs are checked and annotated with the release that
# `npm create vibelang@latest` installs: the 0.0.2 native compiler and runtime from
# the release tarballs (VIBELANG_RELEASE_TARBALLS, else dist-npm/tarballs, else the
# registry's own files through npm pack), not this checkout's build.
# - Every docs example must compile with it. VIBELANG_SKIP_RELEASE_SNIPPETS=1 skips
#   that check, and needs VIBELANG_SKIP_RELEASE_SNIPPETS_REASON, which is printed.
# - Code blocks get its hovers (docs/tools/hovers.ts). VIBELANG_DOCS_HOVERS=0 turns
#   them off; the default is on, so every redeploy ships the same site.
release=${VIBELANG_RELEASE_VERSION:-0.0.2}
tarballs=${VIBELANG_RELEASE_TARBALLS:-$root/dist-npm/tarballs}
hovers=${VIBELANG_DOCS_HOVERS:-1}
case "$hovers" in
	0 | 1) export VIBELANG_DOCS_HOVERS="$hovers" ;;
	*) echo "deploy-docs: VIBELANG_DOCS_HOVERS must be 0 or 1, not $hovers" >&2; exit 2 ;;
esac
check_snippets=true
if [ "${VIBELANG_SKIP_RELEASE_SNIPPETS:-}" = 1 ]; then
	echo "deploy-docs: skipping the $release release snippet check: ${VIBELANG_SKIP_RELEASE_SNIPPETS_REASON:?deploy-docs: set VIBELANG_SKIP_RELEASE_SNIPPETS_REASON to say why}"
	check_snippets=false
fi
if $check_snippets || [ "$hovers" = 1 ]; then
	case "$(uname -s)-$(uname -m)" in
		Darwin-arm64) platform=darwin-arm64 ;;
		Darwin-x86_64) platform=darwin-x64 ;;
		Linux-aarch64 | Linux-arm64) platform=linux-arm64 ;;
		Linux-x86_64) platform=linux-x64 ;;
		*) echo "deploy-docs: no $release release compiler for $(uname -s)-$(uname -m)" >&2; exit 1 ;;
	esac
	release_dir=$(mktemp -d "${TMPDIR:-/tmp}/vibelang-release.XXXXXX")
	trap 'rm -rf "$release_dir"' EXIT
	# A machine without the release build checks against what npm serves users.
	if [ -z "${VIBELANG_RELEASE_TARBALLS:-}" ] && [ ! -f "$tarballs/vibelang-$platform-$release.tgz" ]; then
		tarballs="$release_dir/npm"
		mkdir -p "$tarballs"
		(cd "$tarballs" && npm pack --silent "vibelang-$platform@$release" "vibelang-runtime@$release" >/dev/null 2>&1) ||
			{ echo "deploy-docs: no $release tarballs in dist-npm/tarballs, and npm has no vibelang-$platform@$release; set VIBELANG_RELEASE_TARBALLS" >&2; exit 1; }
	fi
	for name in "vibelang-$platform" vibelang-runtime; do
		mkdir -p "$release_dir/$name"
		tar xzf "$tarballs/$name-$release.tgz" -C "$release_dir/$name" 2>/dev/null ||
			{ echo "deploy-docs: missing $tarballs/$name-$release.tgz; set VIBELANG_RELEASE_TARBALLS" >&2; exit 1; }
	done
	# As an npm install puts effect beside vibelang-runtime, give the extracted runtime
	# the repository's pinned effect, so its declarations resolve their imports.
	mkdir -p "$release_dir/vibelang-runtime/package/node_modules"
	ln -s "$(cd "$root/packages/runtime/node_modules/effect" && pwd -P)" "$release_dir/vibelang-runtime/package/node_modules/effect"
	release_binary="$release_dir/vibelang-$platform/package/bin/vibelang"
	release_runtime="$release_dir/vibelang-runtime/package"
fi
if $check_snippets; then
	echo "deploy-docs: checking the starter output and tests with vibelang $release"
	VIBELANG_BINARY="$release_binary" VIBELANG_RUNTIME_DIR="$release_runtime" \
		bun test "$root/docs/tools/first-run.test.ts" > "$release_dir/first-run.log" 2>&1 ||
		{ tail -40 "$release_dir/first-run.log" >&2; echo "deploy-docs: the starter does not match its documented behavior" >&2; exit 1; }
	echo "deploy-docs: compiling docs examples and checking declared output with vibelang $release ($platform) and vibelang-runtime $release"
	VIBELANG_BINARY="$release_binary" VIBELANG_RUNTIME_DIR="$release_runtime" \
		bun "$root/docs/tools/check-snippets.ts" --run > "$release_dir/snippets.log" ||
		{ tail -40 "$release_dir/snippets.log" >&2; echo "deploy-docs: a docs example fails compilation or declared output with the $release release" >&2; exit 1; }
	tail -1 "$release_dir/snippets.log"
fi
if [ "$hovers" = 1 ]; then
	echo "deploy-docs: building code-block hovers with vibelang $release"
	VIBELANG_BINARY="$release_binary" VIBELANG_RUNTIME_DIR="$release_runtime" \
		bun "$root/docs/tools/hovers.ts" > "$release_dir/hovers.log" 2>&1 ||
		{ tail -40 "$release_dir/hovers.log" >&2; echo "deploy-docs: building hovers failed; VIBELANG_DOCS_HOVERS=0 deploys without them" >&2; exit 1; }
	head -1 "$release_dir/hovers.log"
fi
echo "deploy-docs: checking the published token counts"
bun "$root/docs/tools/token-counts.ts" --check
# The docs Worker (docs/wrangler.jsonc) serves effeck.dev, so this build's links name
# effeck.dev, where its Markdown copies live. vibelang.sh is a separate site today.
export VIBELANG_DOCS_ORIGIN="${VIBELANG_DOCS_ORIGIN:-https://effeck.dev}"
VIBELANG_BINARY="${release_binary:-vibelang}" VIBELANG_RUNTIME_DIR="${release_runtime:-$root/packages/runtime}" \
	bun "$root/docs/tools/render-user.ts"

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
