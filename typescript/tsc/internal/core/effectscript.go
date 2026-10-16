package core

import "maps"

// EffectScriptOptions is compilerOptions.effectscript in tsconfig.json, the
// project's EffectScript settings (design decision D34).
type EffectScriptOptions struct {
	// Diagnostics maps an ES code whose category a project may change, such as
	// "ES2004", to "error", "warning", or "off".
	Diagnostics map[string]string `json:"diagnostics,omitzero"`
	// Tracing is "none", "exported", or "all": which functions compile with
	// Effect.fn spans. Empty means "none".
	Tracing string `json:"tracing,omitzero"`
}

// Equals reports whether o and other hold the same settings.
func (o *EffectScriptOptions) Equals(other *EffectScriptOptions) bool {
	if o == nil || other == nil {
		return o == other
	}
	return o.Tracing == other.Tracing && maps.Equal(o.Diagnostics, other.Diagnostics)
}
