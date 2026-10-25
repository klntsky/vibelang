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

test('user prose drops decision annotations without damaging diagnostics, proposal links, or code', () => {
  const page = render([
    '# Example', '', 'Examples (D126).',
    '**Classes** (D278, D304). Block params (block params, D188).',
    'A warning (`EK4402`, D108). Range (D284-D290).',
    '**Throw** (D186, [proposal](https://example.com/throw)).',
    'Keep inline `D126` and version D3D12.', '',
    '```ts', 'const D126 = "(D126)"', '// Output: D126', '```', ''
  ].join('\n'))
  assert.match(page, /Examples\./)
  assert.match(page, /\*\*Classes\*\*\./)
  assert.match(page, /\(block params\)/)
  assert.match(page, /\(`EK4402`\)/)
  assert.match(page, /Range\./)
  assert.match(page, /\[proposal\]\(https:\/\/example.com\/throw\)/)
  assert.match(page, /Keep inline `D126` and version D3D12/)
  assert.match(page, /const D126 = "\(D126\)"/)
  assert.doesNotMatch(page, /D278|D304|D188|D108|D284|D290|D186/)
})
