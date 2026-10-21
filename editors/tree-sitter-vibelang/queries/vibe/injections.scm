; inherits: typescript

; VibeLang adds no injections: comments, JSDoc, regular expressions, and
; template strings inject as in TypeScript. A module declaration's body (D283)
; is TypeScript, but it needs no typescript injection: this grammar gives it
; tree-sitter-typescript's nodes, which the inherited TypeScript queries color.
