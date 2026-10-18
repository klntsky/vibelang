package core

import "maps"

// EffeckOptions is compilerOptions.effeck in tsconfig.json, the
// project's Effeck settings (design decision D34).
type EffeckOptions struct {
	// Diagnostics maps an EK code in EffeckAdjustableCodes, such as "EK2004",
	// to "error", "warning", or "off".
	Diagnostics map[string]string `json:"diagnostics,omitzero"`
	// Tracing is "none", "exported", or "all": which functions compile with
	// Effect.fn spans. Empty means "none".
	Tracing string `json:"tracing,omitzero"`
}

// EffeckAdjustableCodes are the EK codes whose category compilerOptions.effeck
// and @effeck-diagnostics comments may change (D34, D241): EK2004, EK4403,
// and the rules taken from Effect's language service, EK4801-EK4825, except
// EK4813, which stands in for a TypeScript error.
var EffeckAdjustableCodes = []string{
	"EK2004", "EK4403",
	"EK4801", "EK4802", "EK4803", "EK4804", "EK4805", "EK4806", "EK4807", "EK4808", "EK4809", "EK4810",
	"EK4811", "EK4812", "EK4814", "EK4815", "EK4816", "EK4817", "EK4818", "EK4819", "EK4820",
	"EK4821", "EK4822", "EK4823", "EK4824", "EK4825",
}

// Equals reports whether o and other hold the same settings.
func (o *EffeckOptions) Equals(other *EffeckOptions) bool {
	if o == nil || other == nil {
		return o == other
	}
	return o.Tracing == other.Tracing && maps.Equal(o.Diagnostics, other.Diagnostics)
}
