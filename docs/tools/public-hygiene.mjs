// Structural public-repository checks, shared with the tracked-file leak gate.
const retiredService = ["converter", "worker"].join("-")

export function publicHygiene(file, text) {
  const problems = []
  if (file.includes(retiredService) || text.includes(retiredService)) {
    problems.push("a removed hosted compile service")
  }
  const scratchRoots = ["review-codex", "review-fable", "agent-a", "abstract-services"].map(name => "scratch-" + name)
  const removedPaths = ["docs/" + "reviews/", ...scratchRoots.map(name => "experiments/" + name + "/"),
    "artifacts/" + "effeck-" + "design-space.html"] // brand-gate: allow
  if (removedPaths.some(path => file.startsWith(path) || text.includes(path)) ||
      scratchRoots.some(name => text.includes(name + "/"))) {
    problems.push("a removed review artifact or scratch path")
  }
  const spec = file.startsWith("docs/design/src/") || file === "docs/design/DECISIONS.md" ||
    file.startsWith("site/src/pages/spec/")
  if (spec && (/\b(?:Codex|Fable|GPT-\d|Will)\b|effectscript-4a|Smithers twin|will-direct-session|owner.s decisions/.test(text) ||
      /review probe R\d|(?:prototype )?finding \d+|VibeLang language lead|review, (?:finding|must-fix) \d/i.test(text))) {
    problems.push("internal decision or review provenance")
  }
  return problems
}
