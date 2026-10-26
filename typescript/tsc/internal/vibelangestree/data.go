package vibelangestree

import (
	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/parser"
)

// A data declaration (D307) converts as the class its source reads as, so
// that scope analysis declares its name as a value and a type, as the
// checker does:
//
//	data Delivery<T> {                      class Delivery<T> {
//	  Waiting                                 Waiting
//	  Sent { tracking: T }                    Sent: { tracking: T }
//	}                                       }
//
// Each variant is a property whose type annotation is its fields, written
// without the colon. The constant and the interface the parser writes beside
// the type alias are left out (skipStatement).

// dataDeclaration converts n, a data declaration's type alias.
func (c *converter) dataDeclaration(n *ast.Node) *obj {
	properties := []any{}
	for _, variant := range parser.DataVariants(n) {
		name := variant.Name()
		end := name.End()
		property := c.nodeAt(span{c.start(name), end}, "PropertyDefinition").
			set("accessibility", nil).
			set("computed", false).
			set("declare", false).
			set("decorators", []any{}).
			set("definite", false).
			set("key", c.convertChild(name, variant)).
			set("optional", false).
			set("override", false).
			set("readonly", false).
			set("static", false).
			set("value", nil)
		if fields := variant.Type(); fields != nil && !synthesized(fields) {
			property.set("typeAnnotation", c.nodeAt(c.rangeOf(fields), "TSTypeAnnotation").set("typeAnnotation", c.convertChild(fields, variant)))
			property.setRange(span{c.start(name), fields.End()})
		}
		properties = append(properties, property)
	}
	literal := n.Type().TypeArguments()[0]
	body := c.node(literal, "ClassBody").set("body", properties)
	result := c.node(n, "ClassDeclaration").
		set("abstract", false).
		set("body", body).
		set("declare", hasModifier(ast.KindDeclareKeyword, n)).
		set("decorators", []any{}).
		set("id", c.convertChild(n.Name(), n)).
		set("implements", []any{}).
		set("superClass", nil)
	setTypeParameters(result, c.convertTypeParameters(n.TypeParameterList(), n))
	return c.fixExports(n, result)
}
