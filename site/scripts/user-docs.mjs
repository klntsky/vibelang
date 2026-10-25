// Publication-only cleanup. Source comments remain available to the snippet harness.
export function stripRunDirectives(markdown) {
  let fence
  return markdown.split('\n').filter((line) => {
    const bare = line.trimStart()
    const marker = /^(`{3,}|~{3,})/.exec(bare)?.[1]
    if (fence === undefined) {
      if (marker) fence = marker
      return true
    }
    if (marker?.[0] === fence[0] && marker.length >= fence.length && bare.trimEnd() === marker) {
      fence = undefined
      return true
    }
    return !/^\s*\/\/\s*run:/.test(line)
  }).join('\n')
}
