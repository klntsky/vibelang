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

// compilerOptions.effectscript holds a project's EffectScript settings (design
// decision D34):
//
//	"effectscript": {
//	  "diagnostics": { "ES2004": "off", "ES4403": "warning" },
//	  "tracing": "exported"
//	}
//
// It is added by hand to the generated option tables: its declaration in
// declarations_generated.go; its parse, merge, comparison, and serialization
// cases in options_generated.go; and core.CompilerOptions.EffectScript.

var (
	// effectScriptAdjustableCodes are the ES codes whose category a project may
	// change. Every other ES diagnostic is an error.
	effectScriptAdjustableCodes = []string{"ES2004", "ES4403"}
	effectScriptCategories      = []string{"error", "warning", "off"}
	effectScriptTracing         = []string{"none", "exported", "all"}
)

// parseEffectScriptOptions reads a compilerOptions.effectscript value that
// validateEffectScriptOptions accepted.
func parseEffectScriptOptions(value any) *core.EffectScriptOptions {
	object, ok := value.(*collections.OrderedMap[string, any])
	if !ok {
		return nil
	}
	options := &core.EffectScriptOptions{}
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

// validateEffectScriptOptions reports each unknown key and invalid value in a
// compilerOptions.effectscript value, at its node in the tsconfig file.
func validateEffectScriptOptions(value any, valueExpression *ast.Expression, sourceFile *ast.SourceFile) []*ast.Diagnostic {
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
				report(jsonInitializer(property), diagnostics.Compiler_option_0_requires_a_value_of_type_1, "effectscript.diagnostics", "object")
				continue
			}
			for code, category := range codes.Entries() {
				codeProperty := jsonProperty(jsonInitializer(property), code)
				if !slices.Contains(effectScriptAdjustableCodes, code) {
					d := report(jsonName(codeProperty), diagnostics.Unknown_compiler_option_0, "effectscript.diagnostics."+code)
					d.AddMessageChain(ast.NewExternalDiagnostic(nil, core.TextRange{}, "", diagnostics.CategoryMessage, 0,
						"Only ES2004 and ES4403 can be configured; every other ES diagnostic is an error."))
					continue
				}
				if category, ok := category.(string); !ok || !slices.Contains(effectScriptCategories, category) {
					report(jsonInitializer(codeProperty), diagnostics.Argument_for_0_option_must_be_Colon_1, "effectscript.diagnostics."+code, quoteList(effectScriptCategories))
				}
			}
		case "tracing":
			if tracing, ok := v.(string); !ok || !slices.Contains(effectScriptTracing, tracing) {
				report(jsonInitializer(property), diagnostics.Argument_for_0_option_must_be_Colon_1, "effectscript.tracing", quoteList(effectScriptTracing))
			}
		default:
			report(jsonName(property), diagnostics.Unknown_compiler_option_0, "effectscript."+key)
		}
	}
	return errors
}

// serializeEffectScriptOptions writes options as tsconfig JSON for --showConfig.
func serializeEffectScriptOptions(options *core.EffectScriptOptions) *collections.OrderedMap[string, any] {
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
