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
  return problems
}
