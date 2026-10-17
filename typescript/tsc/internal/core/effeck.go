package core

import "maps"

// EffeckOptions is compilerOptions.effeck in tsconfig.json, the
// project's Effeck settings (design decision D34).
type EffeckOptions struct {
	// Diagnostics maps an ES code whose category a project may change, such as
	// "EK2004", to "error", "warning", or "off".
	Diagnostics map[string]string `json:"diagnostics,omitzero"`
	// Tracing is "none", "exported", or "all": which functions compile with
	// Effect.fn spans. Empty means "none".
	Tracing string `json:"tracing,omitzero"`
}

// Equals reports whether o and other hold the same settings.
func (o *EffeckOptions) Equals(other *EffeckOptions) bool {
	if o == nil || other == nil {
		return o == other
	}
	return o.Tracing == other.Tracing && maps.Equal(o.Diagnostics, other.Diagnostics)
}
