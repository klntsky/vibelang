import assert from 'node:assert/strict'
import { cpSync, mkdtempSync, mkdirSync, readFileSync, rmSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { spawnSync } from 'node:child_process'
import { test } from 'node:test'

function render(markdown) {
  const root = mkdtempSync(join(tmpdir(), 'vibelang-user-docs-'))
  try {
    cpSync(new URL('.', import.meta.url), join(root, 'site/scripts'), { recursive: true })
    mkdirSync(join(root, 'site/src/pages'), { recursive: true })
    mkdirSync(join(root, 'docs/user/src'), { recursive: true })
    writeFileSync(join(root, 'docs/user/inventory.md'), '## V4 sidebar in order\n### Guides\n- [Example](src/example.md)\n')
    writeFileSync(join(root, 'docs/user/src/example.md'), markdown)
    const result = spawnSync(process.execPath, [join(root, 'site/scripts/sync-user-docs.mjs'), join(root, 'docs/user')], { encoding: 'utf8' })
    assert.equal(result.status, 0, result.stderr)
    return readFileSync(join(root, 'site/src/pages/example.mdx'), 'utf8')
  } finally { rmSync(root, { recursive: true, force: true }) }
}

test('rendered user docs hide harness directives in paired and single examples, preserving output and code', () => {
  const page = render([
    '# Example', '', 'A runnable example.', '',
    '```ts effect', '// run: console.log(value)', 'const value = 1', '// Output: 1', '```', '',
    '```vibe', '// run: Console.log(value)', 'const value = 1', '// Output: 1', '```', '',
    '  ~~~ts', '  // run: console.log(text)', '  const text = "// run: literal"', '  // Output: text', '  ~~~', '',
    'The prose marker // run: is ordinary text.', ''
  ].join('\n'))
  assert.doesNotMatch(page, /^\s*\/\/ run:/m)
  assert.match(page, /:::code-group/)
  assert.match(page, /\/\/ Output: 1/)
  assert.match(page, /\/\/ Output: text/)
  assert.match(page, /const text = "\/\/ run: literal"/)
  assert.match(page, /The prose marker \/\/ run: is ordinary text/)
})
