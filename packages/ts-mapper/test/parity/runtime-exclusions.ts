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
