package tsoptions

import (
	"maps"
	"slices"
	"strings"

	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/collections"
	"github.com/microsoft/TypeScript/tsc/internal/core"
	"github.com/microsoft/TypeScript/tsc/internal/diagnostics"
)

// compilerOptions.effeck holds a project's Effeck settings (design
// decision D34):
//
//	"effeck": {
//	  "diagnostics": { "EK2004": "off", "EK4403": "warning" },
//	  "tracing": "exported"
//	}
//
// It is added by hand to the generated option tables: its declaration in
// declarations_generated.go; its parse, merge, comparison, and serialization
// cases in options_generated.go; and core.CompilerOptions.Effeck.

var (
	// effectScriptAdjustableCodes are the ES codes whose category a project may
	// change. Every other EK diagnostic is an error.
	effectScriptAdjustableCodes = []string{"EK2004", "EK4403"}
	effectScriptCategories      = []string{"error", "warning", "off"}
	effectScriptTracing         = []string{"none", "exported", "all"}
)

// parseEffeckOptions reads a compilerOptions.effeck value that
// validateEffeckOptions accepted.
func parseEffeckOptions(value any) *core.EffeckOptions {
	object, ok := value.(*collections.OrderedMap[string, any])
	if !ok {
		return nil
	}
	options := &core.EffeckOptions{}
	if codes, ok := object.GetOrZero("diagnostics").(*collections.OrderedMap[string, any]); ok {
		for code, category := range codes.Entries() {
			if category, ok := category.(string); ok && slices.Contains(effectScriptAdjustableCodes, code) && slices.Contains(effectScriptCategories, category) {
				if options.Diagnostics == nil {
					options.Diagnostics = make(map[string]string)
				}
				options.Diagnostics[code] = category
			}
		}
	}
	if tracing, ok := object.GetOrZero("tracing").(string); ok && slices.Contains(effectScriptTracing, tracing) {
		options.Tracing = tracing
	}
	return options
}

// validateEffeckOptions reports each unknown key and invalid value in a
// compilerOptions.effeck value, at its node in the tsconfig file.
func validateEffeckOptions(value any, valueExpression *ast.Expression, sourceFile *ast.SourceFile) []*ast.Diagnostic {
	object, ok := value.(*collections.OrderedMap[string, any])
	if !ok {
		return nil // convertJsonOption already reported that the value is not an object
	}
	var errors []*ast.Diagnostic
	report := func(node *ast.Node, message *diagnostics.Message, args ...any) *ast.Diagnostic {
		d := CreateDiagnosticForNodeInSourceFileOrCompilerDiagnostic(sourceFile, node, message, args...)
		errors = append(errors, d)
		return d
	}
	for key, v := range object.Entries() {
		property := jsonProperty(valueExpression, key)
		switch key {
		case "diagnostics":
			codes, ok := v.(*collections.OrderedMap[string, any])
			if !ok {
				report(jsonInitializer(property), diagnostics.Compiler_option_0_requires_a_value_of_type_1, "effeck.diagnostics", "object")
				continue
			}
			for code, category := range codes.Entries() {
				codeProperty := jsonProperty(jsonInitializer(property), code)
				if !slices.Contains(effectScriptAdjustableCodes, code) {
					d := report(jsonName(codeProperty), diagnostics.Unknown_compiler_option_0, "effeck.diagnostics."+code)
					d.AddMessageChain(ast.NewExternalDiagnostic(nil, core.TextRange{}, "", diagnostics.CategoryMessage, 0,
						"Only EK2004 and EK4403 can be configured; every other EK diagnostic is an error."))
					continue
				}
				if category, ok := category.(string); !ok || !slices.Contains(effectScriptCategories, category) {
					report(jsonInitializer(codeProperty), diagnostics.Argument_for_0_option_must_be_Colon_1, "effeck.diagnostics."+code, quoteList(effectScriptCategories))
				}
			}
		case "tracing":
			if tracing, ok := v.(string); !ok || !slices.Contains(effectScriptTracing, tracing) {
				report(jsonInitializer(property), diagnostics.Argument_for_0_option_must_be_Colon_1, "effeck.tracing", quoteList(effectScriptTracing))
			}
		default:
			report(jsonName(property), diagnostics.Unknown_compiler_option_0, "effeck."+key)
		}
	}
	return errors
}

// serializeEffeckOptions writes options as tsconfig JSON for --showConfig.
func serializeEffeckOptions(options *core.EffeckOptions) *collections.OrderedMap[string, any] {
	result := collections.NewOrderedMapWithSizeHint[string, any](2)
	if len(options.Diagnostics) > 0 {
		codes := collections.NewOrderedMapWithSizeHint[string, any](len(options.Diagnostics))
		for _, code := range slices.Sorted(maps.Keys(options.Diagnostics)) {
			codes.Set(code, options.Diagnostics[code])
		}
		result.Set("diagnostics", codes)
	}
	if options.Tracing != "" {
		result.Set("tracing", options.Tracing)
	}
	return result
}

// jsonProperty returns the property named name in object, a JSON object
// literal, or nil.
func jsonProperty(object *ast.Node, name string) *ast.Node {
	if object == nil || !ast.IsObjectLiteralExpression(object) {
		return nil
	}
	for _, property := range object.AsObjectLiteralExpression().Properties.Nodes {
		if ast.IsPropertyAssignment(property) && property.Name() != nil && property.Name().Text() == name {
			return property
		}
	}
	return nil
}

func jsonName(property *ast.Node) *ast.Node {
	if property == nil {
		return nil
	}
	return property.Name()
}

func jsonInitializer(property *ast.Node) *ast.Node {
	if property == nil {
		return nil
	}
	return property.Initializer()
}

func quoteList(values []string) string {
	return "'" + strings.Join(values, "', '") + "'"
}
