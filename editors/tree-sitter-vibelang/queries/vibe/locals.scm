; inherits: typescript

; A match arm's bindings are its own (D206).
(match_arm) @local.scope

(handle_clause) @local.scope

(binding_pattern
  name: (identifier) @local.definition.var)

; finally (exit) binds the exit (D178).
(finally_clause
  parameter: (identifier) @local.definition.parameter)
