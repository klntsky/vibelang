# super and arguments in Effect code

An Effect method's body runs in a generator, and so do an Effect arrow's body
and a try statement's regions. JavaScript binds super and arguments afresh in
each generator: super there is a SyntaxError and arguments the generator's
own. The method declares a reader of its super and a copy of its arguments,
and the generated code uses those: an error family's subclass calling its
base, a class's instance, accessor, element and static super reads, plain, compound and logical assignments to a super accessor, super in
an Effect arrow and in try and finally, and arguments in a method, a region
and an arrow inside a function expression.
