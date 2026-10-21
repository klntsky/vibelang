// Parses the VibeLang corpus with the vibe grammar and fails when any file has
// an ERROR or MISSING node.
//
//   node scripts/corpus.mjs [--report] [--lib <parser.so> --lang-name <name>] [group=path ...]
//
// The corpus is the repository's .vibe files (the compiler's cases without the
// failing ones, which end in .fail, in the group cases; the rest, without the
// files in invalid below, in repo) and every ```vibe block of the Markdown
// under docs/ (docs). A group=path argument
// adds the .vibe files under path as another group, such as
// fs=~/smithers/packages/smithers/agent/fs.
//
// For each group the script prints the files and lines inside ERROR or MISSING
// nodes, then each file that has one, with the lines. --report prints the
// counts and exits 0. --lib and --lang-name parse with another compiled parser,
// such as tree-sitter-typescript's, to measure it on the same corpus.
import { execFileSync } from 'node:child_process';
import { mkdtempSync, readdirSync, readFileSync, rmSync, statSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { dirname, join, relative, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const grammarDir = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const repo = resolve(grammarDir, '../..');
const cli = join(grammarDir, 'node_modules', '.bin', 'tree-sitter');

const args = process.argv.slice(2);
const report = args.includes('--report');
const option = (name) => {
  const at = args.indexOf(name);
  return at >= 0 ? args[at + 1] : undefined;
};
const lib = option('--lib');
const langName = option('--lang-name');
const extra = args.filter((arg, i) => /^[\w-]+=/.test(arg) && !['--lib', '--lang-name'].includes(args[i - 1]));

const skipped = new Set(['node_modules', '.git', '.jj']);

// Files that are not VibeLang on purpose.
const invalid = new Set([
  'packages/compiler/test/eslint/fixtures/broken.vibe', // ESLint's parse error fixture
]);

function vibeFiles(dir, out = []) {
  for (const entry of readdirSync(dir, { withFileTypes: true })) {
    if (skipped.has(entry.name)) continue;
    const path = join(dir, entry.name);
    if (entry.isDirectory()) vibeFiles(path, out);
    else if (entry.name.endsWith('.vibe')) out.push(path);
  }
  return out;
}

function markdownFiles(dir, out = []) {
  for (const entry of readdirSync(dir, { withFileTypes: true })) {
    if (skipped.has(entry.name)) continue;
    const path = join(dir, entry.name);
    if (entry.isDirectory()) markdownFiles(path, out);
    else if (entry.name.endsWith('.md')) out.push(path);
  }
  return out;
}

// docBlocks writes each ```vibe block of the Markdown under docs/ to scratch,
// named after its file and line, and returns the paths it wrote.
function docBlocks(scratch) {
  const paths = [];
  for (const file of markdownFiles(join(repo, 'docs'))) {
    const lines = readFileSync(file, 'utf8').split(/\r?\n/);
    for (let i = 0; i < lines.length; i++) {
      const fence = /^\s{0,3}(`{3,}|~{3,})\s*vibe(?:\s|$)/.exec(lines[i]);
      if (!fence) continue;
      const start = i + 1;
      const close = new RegExp(`^\\s{0,3}${fence[1][0]}{${fence[1].length},}\\s*$`);
      const body = [];
      while (++i < lines.length && !close.test(lines[i])) body.push(lines[i]);
      const name = `${relative(repo, file).replace(/[/\\]/g, '__')}__L${start}.vibe`;
      const path = join(scratch, name);
      writeFileSync(path, `${body.join('\n')}\n`);
      paths.push(path);
    }
  }
  return paths;
}

// parse returns, for each path, the rows its ERROR and MISSING nodes cover.
function parse(paths, scratch) {
  if (paths.length === 0) return [];
  const list = join(scratch, 'paths.txt');
  writeFileSync(list, paths.join('\n'));
  const parserArgs = lib ? ['--lib-path', lib, '--lang-name', langName ?? 'vibe'] : [];
  let output;
  try {
    output = execFileSync(cli, ['parse', ...parserArgs, '--paths', list], {
      cwd: grammarDir,
      encoding: 'utf8',
      maxBuffer: 1 << 30,
      stdio: ['ignore', 'pipe', 'ignore'],
    });
  } catch (error) {
    // tree-sitter parse exits 1 when a file has an error; its trees are still on stdout.
    if (typeof error.stdout !== 'string') throw error;
    output = error.stdout;
  }
  // Each tree starts at column 0 with its root: program, or ERROR when nothing parsed.
  const trees = output.split(/^(?=\()/m).filter((tree) => tree.startsWith('('));
  if (trees.length !== paths.length) {
    throw new Error(`tree-sitter parse printed ${trees.length} trees for ${paths.length} files`);
  }
  return trees.map((tree) => {
    const rows = new Set();
    for (const match of tree.matchAll(/\((ERROR|MISSING)\b[^[]*\[(\d+), \d+\] - \[(\d+), \d+\]/g)) {
      const first = Number(match[2]);
      const last = match[1] === 'MISSING' ? first : Number(match[3]);
      for (let row = first; row <= last; row++) rows.add(row);
    }
    return [...rows].sort((a, b) => a - b);
  });
}

// ranges writes zero-based rows as one-based line ranges: 3-5 9.
function ranges(rows) {
  const out = [];
  for (let i = 0; i < rows.length; i++) {
    let j = i;
    while (j + 1 < rows.length && rows[j + 1] === rows[j] + 1) j++;
    out.push(i === j ? `${rows[i] + 1}` : `${rows[i] + 1}-${rows[j] + 1}`);
    i = j;
  }
  return out.join(' ');
}

function lineCount(path) {
  const text = readFileSync(path, 'utf8');
  return text.split('\n').length - (text.endsWith('\n') ? 1 : 0);
}

const scratch = mkdtempSync(join(tmpdir(), 'vibe-corpus-'));
let failed = false;
try {
  const all = vibeFiles(repo);
  const isCase = (path) => relative(repo, path).startsWith(join('typescript', 'vibelang', 'cases') + '/');
  const isFailingCase = (path) => /(^|\/)[^/]+\.fail\//.test(relative(join(repo, 'typescript', 'vibelang', 'cases'), path));
  const groups = [
    ['cases', all.filter((path) => isCase(path) && !isFailingCase(path))],
    ['repo', all.filter((path) => !isCase(path) && !invalid.has(relative(repo, path)))],
    ['docs', docBlocks(scratch)],
  ];
  for (const arg of extra) {
    const at = arg.indexOf('=');
    const dir = resolve(arg.slice(at + 1).replace(/^~(?=\/|$)/, process.env.HOME ?? '~'));
    groups.push([arg.slice(0, at), statSync(dir).isDirectory() ? vibeFiles(dir) : [dir]]);
  }
  for (const [name, paths] of groups) {
    const results = parse(paths, scratch);
    let lines = 0;
    let badLines = 0;
    const bad = [];
    paths.forEach((path, i) => {
      lines += lineCount(path);
      badLines += results[i].length;
      if (results[i].length) bad.push([path, results[i]]);
    });
    console.log(`${name}: ${bad.length} of ${paths.length} files, ${badLines} of ${lines} lines in ERROR or MISSING`);
    for (const [path, rows] of bad) {
      const shown = path.startsWith(scratch) ? relative(scratch, path) : relative(repo, path);
      console.log(`  ${shown}: lines ${ranges(rows)}`);
    }
    if (bad.length) failed = true;
  }
} finally {
  rmSync(scratch, { recursive: true, force: true });
}
process.exit(failed && !report ? 1 : 0);
