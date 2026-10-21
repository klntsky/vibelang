# D282: plain functions catch typed failures only

A plain function may throw what its own catch handles, and it still returns a
value. Its catch receives typed failures only, with their types: a throw and a
trusted call's listed failure. A TypeScript function's exception passes
through the catch unchanged, as a defect would, and so does a trusted call's
exception outside a try. Under an Effect contract the
same throws, in a statement, a match arm, a do block, or a conditional, fail
the Effect.
