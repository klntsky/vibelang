# D296: a plain generator never fails

A generator with a JavaScript iterator annotation (D271) follows D282 like any
plain function: a throw that can escape it, sync or async, is EK2003. A throw
its own catch handles stays inside, and the catch binding has its type.
