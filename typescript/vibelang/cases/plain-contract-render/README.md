# D282: B-4

Smithers twin finding B-4 threw a JavaScript TypeError from a plain-annotated
renderer. Under D282 a `throw` outside a hatch is a typed failure, so the
JavaScript throw moves into the `sync` hatch, which makes the exception a
defect, and the function's contract is `Effect<string>`.
