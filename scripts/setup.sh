#!/bin/sh
# Local, unpublished packages. BUN_CREATE_DIR and EFFECTSCRIPT_BIN_DIR let tests
# isolate template and launcher installation without changing HOME.
set -eu
root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
for tool in go bun node; do
  command -v "$tool" >/dev/null 2>&1 || { echo "setup: install $tool and rerun sh scripts/setup.sh" >&2; exit 1; }
done
echo "setup: Node $(node --version), Bun $(bun --version), $(go version)"
sh "$root/scripts/build-esc.sh"

if [ -f "$root/packages/runtime/package.json" ]; then
  echo "setup: install runtime dependencies"
  (cd "$root/packages/runtime" && bun install)
  if node --input-type=module -e 'import fs from "node:fs"; process.exit(JSON.parse(fs.readFileSync(process.argv[1])).scripts?.build ? 0 : 1)' "$root/packages/runtime/package.json"; then
    echo "setup: build runtime"
    (cd "$root/packages/runtime" && bun run build)
  else
    echo "setup: runtime has no build script; using its declared exports"
  fi
else
  echo "setup: WARNING: packages/runtime is absent. The compiler is ready; projects need the runtime lane before they can install." >&2
fi

node "$root/scripts/install-template.mjs"

bin_dir=${EFFECTSCRIPT_BIN_DIR:-$HOME/.local/bin}
case ":$PATH:" in
  *":$bin_dir:"*)
    mkdir -p "$bin_dir"
    if [ -e "$bin_dir/esc" ] || [ -L "$bin_dir/esc" ]; then
      if [ ! -L "$bin_dir/esc" ] || [ "$(readlink "$bin_dir/esc")" != "$root/packages/compiler/bin/esc" ]; then
        echo "setup: preserving existing $bin_dir/esc; use the project's bun run build"
      fi
    else
      ln -s "$root/packages/compiler/bin/esc" "$bin_dir/esc"
      echo "setup: linked $bin_dir/esc"
    fi
    ;;
  *)
    echo "setup: $bin_dir is not on PATH. Project scripts find esc after bun install."
    echo "setup: for a global command, add $bin_dir to PATH and rerun setup."
    ;;
esac

if [ -f "$root/editors/vscode/package.json" ]; then
  echo "setup: build VS Code extension"
  (cd "$root/editors/vscode" && bun install && bun run build)
  # Prefer the package's own packaging script when provided by the editor lane.
  if node --input-type=module -e 'import fs from "node:fs"; process.exit(JSON.parse(fs.readFileSync(process.argv[1])).scripts?.package ? 0 : 1)' "$root/editors/vscode/package.json"; then
    (cd "$root/editors/vscode" && bun run package)
  else
    (cd "$root/editors/vscode" && bunx @vscode/vsce@4.0.0 package --allow-missing-repository)
  fi
  find "$root/editors/vscode" -maxdepth 1 -name '*.vsix' -exec sh -c 'printf "Editor: code --install-extension \"%s\"\nCursor: cursor --install-extension \"%s\"\n" "$1" "$1"' sh {} \;
else
  echo "setup: editors/vscode is absent; rerun setup after the editor lane lands to build its .vsix."
fi

echo "setup: local file dependencies point to $root; keep this checkout available."
printf '\nNext commands:\n  bun create effectscript my-app --no-git\n  cd my-app\n  bun run start\n'
