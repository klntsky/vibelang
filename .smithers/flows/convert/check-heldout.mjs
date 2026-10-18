// Reject gate sources in training examples or the prompt, even under different file names.
import { readFileSync, realpathSync } from "node:fs"
import { join } from "node:path"
import { fileURLToPath } from "node:url"
import { casesIn } from "./cases.mjs"

const here = fileURLToPath(new URL(".", import.meta.url))

// Compare tokens, ignoring comments and spacing between them. Keep literal contents and
// token boundaries: "a b" differs from "ab", as do identifiers `a b` and `ab`.
// Template interpolations are code; template text, strings and regexes preserve comment markers.
export function normalizeSource(source) {
  const tokens = []
  let i = 0
  function quoted(quote) {
    const start = i++
    while (i < source.length) {
      const c = source[i++]
      if (c === "\\") i++
      else if (c === quote) break
    }
    tokens.push(source.slice(start, i))
  }
  function template() {
    tokens.push(source[i++])
    let start = i
    while (i < source.length) {
      if (source[i] === "\\") { i += 2; continue }
      if (source[i] === "`") {
        tokens.push("template:" + source.slice(start, i), source[i++])
        return
      }
      if (source.startsWith("${", i)) {
        tokens.push("template:" + source.slice(start, i), "${")
        i += 2
        code(true)
        start = i
      } else i++
    }
    tokens.push("template:" + source.slice(start, i))
  }
  function code(interpolation = false) {
    let braces = 0
    while (i < source.length) {
      const c = source[i]
      if (/\s/.test(c)) { i++; continue }
      if (source.startsWith("//", i)) {
        while (i < source.length && !/[\r\n]/.test(source[i])) i++
        continue
      }
      if (source.startsWith("/*", i)) {
        const end = source.indexOf("*/", i + 2)
        i = end < 0 ? source.length : end + 2
        continue
      }
      if (c === '"' || c === "'") { quoted(c); continue }
      if (c === "`") { template(); continue }
      if (c === "/" && (tokens.length === 0 || /^(?:[=([{,:;!?]|=>|return|throw|case|yield|\|\||&&|\?\?)$/.test(tokens.at(-1)))) {
        const start = i++
        let inClass = false
        while (i < source.length) {
          const r = source[i++]
          if (r === "\\") i++
          else if (r === "[") inClass = true
          else if (r === "]") inClass = false
          else if (r === "/" && !inClass) break
        }
        while (/[a-z]/i.test(source[i] ?? "")) i++
        tokens.push(source.slice(start, i))
        continue
      }
      const word = /^[\p{ID_Start}\p{ID_Continue}\d_$]+/u.exec(source.slice(i))
      const punctuation = /^(?:===|!==|>>>|\*\*=|&&=|\|\|=|\?\?=|=>|==|!=|<=|>=|\+\+|--|&&|\|\||\?\?|\?\.|\*\*|<<|>>|\+=|-=|\*=|\/=|%=|&=|\|=|\^=|\|>|\.\.\.)/.exec(source.slice(i))
      const token = word?.[0] ?? punctuation?.[0] ?? c
      tokens.push(token)
      i += token.length
      if (c === "{") braces++
      if (c === "}") {
        if (interpolation && braces === 0) return
        braces--
      }
    }
  }
  code()
  return tokens.length ? " " + tokens.map((token) => JSON.stringify(token)).join(" ") + " " : ""
}

function promptSources(prompt) {
  const sources = []
  const lines = prompt.split(/\r?\n/)
  for (let i = 0; i < lines.length; i++) {
    const fence = /^\s{0,3}(`{3,}|~{3,})/.exec(lines[i])
    if (!fence) continue
    const close = new RegExp(`^\\s{0,3}${fence[1][0]}{${fence[1].length},}\\s*$`)
    const body = []
    while (++i < lines.length && !close.test(lines[i])) body.push(lines[i])
    sources.push(body.join("\n"))
  }
  // Also catch an unfenced source pasted directly into prompt.md.
  sources.push(prompt)
  return sources
}

export function heldoutProblems({ golden = casesIn(join(here, "golden")), examples = casesIn(join(here, "examples")), prompt = readFileSync(join(here, "prompt.md"), "utf8") } = {}) {
  const candidates = examples.flatMap((entry) => [
    { location: `examples/${entry.name}/input.ts`, source: entry.input },
    { location: `examples/${entry.name}/expected.vibe`, source: entry.expected }
  ])
  candidates.push(...promptSources(prompt).map((source) => ({ location: "prompt.md", source })))
  const normalized = candidates.map((entry) => ({ ...entry, source: normalizeSource(entry.source) }))
  const problems = new Set()
  for (const entry of golden) {
    for (const [file, source] of [["input.ts", entry.input], ["expected.vibe", entry.expected]]) {
      const heldout = normalizeSource(source)
      if (!heldout) throw new Error(`golden/${entry.name}/${file} has no source tokens`)
      for (const candidate of normalized) {
        if (candidate.source.includes(heldout)) problems.add(`${candidate.location} overlaps held-out golden/${entry.name}/${file} (normalized source)`)
      }
    }
  }
  return [...problems]
}

// Compare real paths: on macOS tmpdir() is under /var, a symlink to /private/var, and
// import.meta.url names the resolved path while argv[1] keeps the one the caller passed.
if (process.argv[1] && fileURLToPath(import.meta.url) === realpathSync(process.argv[1])) {
  const problems = heldoutProblems()
  if (problems.length) {
    console.error(problems.join("\n"))
    process.exitCode = 1
  } else console.log("check-heldout: examples/ and prompt.md contain no golden source")
}
