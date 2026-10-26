package vibelang

import (
	"slices"
	"strings"

	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/checker"
	"github.com/microsoft/TypeScript/tsc/internal/scanner"
)

// Nested variant tests do not narrow their containing object in TypeScript.
// For diagnostics only, enumerate the finite fields the patterns actually
// test and subtract the arms. Guards and other tests keep the existing type
// display. Bound the product so reporting an error cannot explode in size.
type witnessField struct {
	name  string
	shape *witnessShape
}
type witnessShape struct {
	owner  string
	tags   []string
	fields []witnessField
}
type variantConstraint struct {
	path []string
	tag  string
}
type matchWitness struct {
	name, tag string
	fields    []witnessValue
}
type witnessValue struct {
	name  string
	value matchWitness
}

func nestedMatchRemainder(c *checker.Checker, call *ast.Node, left *checker.Type) string {
	subject := c.GetSymbolAtLocation(call.Arguments()[0])
	if subject == nil {
		return ""
	}
	body := ast.GetContainingFunction(call)
	if body == nil || !ast.IsBlock(body.Body()) {
		return ""
	}
	shape := &witnessShape{}
	var arms [][]variantConstraint
	nested := false
	var test func(*ast.Node, *[]variantConstraint) bool
	test = func(n *ast.Node, constraints *[]variantConstraint) bool {
		n = ast.SkipParentheses(n)
		if ast.IsBinaryExpression(n) && n.AsBinaryExpression().OperatorToken.Kind == ast.KindAmpersandAmpersandToken {
			return test(n.AsBinaryExpression().Left, constraints) && test(n.AsBinaryExpression().Right, constraints)
		}
		if !ast.IsCallExpression(n) || len(n.Arguments()) != 2 || !ast.IsIdentifier(n.Expression()) ||
			n.Expression().Text() != "$is" || n.Expression().Flags&ast.NodeFlagsReparsed == 0 {
			return false
		}
		matcher, reference := n.Arguments()[0], n.Arguments()[1]
		tag, _, ok := dataVariantOf(c, matcher)
		if !ok {
			return false
		}
		var path []string
		for ast.IsPropertyAccessExpression(reference) && ast.IsIdentifier(reference.Name()) {
			path = append(path, reference.Name().Text())
			reference = reference.Expression()
		}
		if !ast.IsIdentifier(reference) || c.GetSymbolAtLocation(reference) != subject {
			return false
		}
		slices.Reverse(path)
		if len(path) > 8 {
			return false
		}
		nested = nested || len(path) != 0
		cursor := shape
		for _, field := range path {
			index := slices.IndexFunc(cursor.fields, func(f witnessField) bool { return f.name == field })
			if index < 0 {
				cursor.fields = append(cursor.fields, witnessField{field, &witnessShape{}})
				index = len(cursor.fields) - 1
			}
			cursor = cursor.fields[index].shape
		}
		owner := scanner.GetTextOfNode(matcher.Expression())
		if cursor.owner != "" && cursor.owner != owner {
			return false
		}
		cursor.owner = owner
		cursor.tags = dataVariants(c, c.GetTypeAtLocation(matcher.Expression()))
		*constraints = append(*constraints, variantConstraint{path, tag})
		return true
	}
	// Each successful return is a separate arm, including when the parser
	// groups several arms in a shared block. Tests on sibling paths are
	// alternatives, not one conjunction.
	var arm func(*ast.Node, []variantConstraint) bool
	arm = func(n *ast.Node, prefix []variantConstraint) bool {
		switch {
		case ast.IsBlock(n):
			for _, statement := range n.Statements() {
				if !arm(statement, prefix) {
					return false
				}
			}
			return true
		case ast.IsIfStatement(n):
			constraints := slices.Clone(prefix)
			return n.AsIfStatement().ElseStatement == nil && test(n.Expression(), &constraints) &&
				arm(n.AsIfStatement().ThenStatement, constraints)
		case ast.IsReturnStatement(n):
			if n.Expression() != call {
				if len(prefix) == 0 {
					return false
				}
				arms = append(arms, prefix)
			}
			return true
		case ast.IsVariableStatement(n):
			return true
		}
		return false
	}
	if !arm(body.Body(), nil) {
		return ""
	}
	if !nested {
		return ""
	}
	candidates, ok := finiteWitnesses(c, left, shape)
	if !ok {
		return ""
	}
	var missing []string
	for _, candidate := range candidates {
		covered := slices.ContainsFunc(arms, func(constraints []variantConstraint) bool {
			return !slices.ContainsFunc(constraints, func(constraint variantConstraint) bool {
				at := &candidate
				for _, field := range constraint.path {
					index := slices.IndexFunc(at.fields, func(f witnessValue) bool { return f.name == field })
					if index < 0 {
						return true
					}
					at = &at.fields[index].value
				}
				return at.tag != constraint.tag
			})
		})
		if !covered {
			missing = append(missing, candidate.text())
		}
	}
	return strings.Join(missing, " | ")
}

func finiteWitnesses(c *checker.Checker, t *checker.Type, shape *witnessShape) ([]matchWitness, bool) {
	var out []matchWitness
	for _, member := range members(t) {
		if member.Flags()&checker.TypeFlagsNever != 0 {
			continue
		}
		candidate := matchWitness{}
		if tag := c.GetTypeOfPropertyOfType(member, "_tag"); tag != nil && tag.IsStringLiteral() && shape.owner != "" {
			candidate.tag = tag.AsLiteralType().Value().(string)
			if !slices.Contains(shape.tags, candidate.tag) {
				return nil, false // a different tagged type shares this union
			}
			candidate.name = shape.owner + "." + candidate.tag
		} else if member.Flags()&checker.TypeFlagsUnit != 0 {
			candidate.name = c.TypeToString(member)
		} else {
			return nil, false
		}
		product := []matchWitness{candidate}
		for _, field := range shape.fields {
			fieldType := c.GetTypeOfPropertyOfType(member, field.name)
			if fieldType == nil {
				continue // this variant has no such field, so that test cannot match
			}
			values, ok := finiteWitnesses(c, fieldType, field.shape)
			if !ok || len(values)*len(product) > 64 {
				return nil, false
			}
			var next []matchWitness
			for _, parent := range product {
				for _, value := range values {
					fields := append(slices.Clone(parent.fields), witnessValue{field.name, value})
					next = append(next, matchWitness{parent.name, parent.tag, fields})
				}
			}
			product = next
		}
		out = append(out, product...)
		if len(out) > 64 {
			return nil, false
		}
	}
	return out, true
}

func (w matchWitness) text() string {
	if len(w.fields) == 0 {
		return w.name
	}
	fields := make([]string, len(w.fields))
	for i, field := range w.fields {
		fields[i] = field.name + ": " + field.value.text()
	}
	return w.name + " { " + strings.Join(fields, "; ") + " }"
}
