import exclusions from "./runtime-exclusions.json"

export type RuntimeExclusion = { id: string; reason: string }

// runtimeExclusions keeps excluded cases visible and detects entries that no
// longer justify exclusion. Missing results in a partial run are not passes.
export function runtimeExclusions(
  knownIds: string[],
  results: { id: string; runtime: { ok: boolean } | null }[],
  listed: RuntimeExclusion[] = exclusions,
) {
  const known = new Set(knownIds)
  const measured = new Map(results.map((r) => [r.id, r.runtime]))
  return {
    exclusions: listed.filter((e) => measured.has(e.id)),
    staleExclusions: listed.flatMap((e) => {
      const detail = !known.has(e.id) ? "listed id does not exist" : measured.get(e.id)?.ok ? "excluded case now passes" : ""
      return detail ? [{ ...e, detail }] : []
    }),
  }
}

// exclusionNote makes every runtime total disclose the cases it omits.
export function exclusionNote(excluded: RuntimeExclusion[]): string {
  return excluded.length ? ` (${excluded.length} excluded: ${excluded.map((e) => `${e.id}, ${e.reason}`).join("; ")})` : ""
}

// typeExclusions measures every declaration before omitting a listed case from
// the concrete column, so fixed cases and misspelled ids cannot hide forever.
export function typeExclusions(
  knownIds: string[],
  results: { id: string; units: { status: string; sourceKind: string; group: string }[] }[],
  listed: RuntimeExclusion[] = exclusions,
) {
  return runtimeExclusions(knownIds, results.map((r) => {
    const declarations = r.units.filter((u) => u.sourceKind !== "ts" && u.group !== "extra")
    return { id: r.id, runtime: { ok: declarations.length > 0 && declarations.every((u) => u.status === "identical" || u.status === "union-order") } }
  }), listed)
}

// typeExclusionNote discloses the concrete column's omitted cases and reasons.
export function typeExclusionNote(excluded: RuntimeExclusion[]): string {
  return excluded.length ? ` (${excluded.length} case${excluded.length === 1 ? "" : "s"} excluded: ${excluded.map((e) => `${e.id}, ${e.reason}`).join("; ")})` : ""
}
