// Probe 2 (not in the corpus): mutual recursion. Each function's contract includes the other's
// failure, so E is a fixpoint over the cycle (FINDINGS.md finding 7). The corpus has only
// self-recursion, where the body alone supplies E and R.
class Odd extends TaggedError("Odd")<{ readonly n: number }> {}

function isEven(n: number) {
  if (n === 0) return true
  return isOdd(n - 1)
}

function isOdd(n: number) {
  if (n === 7) throw new Odd({ n })
  if (n === 0) return false
  return isEven(n - 1)
}

// A caller whose lowering does not depend on the cycle's E.
function check(n: number) {
  return isEven(n)
}
