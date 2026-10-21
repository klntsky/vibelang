# D282: plain contracts and Effect callers

A plain function never fails. It may throw what its own catch handles, and the
catch receives typed failures only, with their types: a throw, a trusted
function's listed failure, and JSON.parse's SyntaxError. Exceptions from
TypeScript code it calls propagate unchanged, and one that escapes into an
Effect caller becomes a defect. A throw that can escape needs an Effect
contract, on a method too.
