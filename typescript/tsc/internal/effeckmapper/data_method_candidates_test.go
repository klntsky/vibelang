package effeckmapper

import (
	"encoding/json"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"gotest.tools/v3/assert"
)

// TestDataMethodCandidateImports pins a unique method's complete call and
// imports, so adding one opaque call cannot retain unrelated module functions.
func TestDataMethodCandidateImports(t *testing.T) {
	source := `import { opaque } from "./opaque.js"; export const value = opaque.isSome();`
	checking := Project(source, testContext)
	assert.Equal(t, virtualBody(t, checking), source)
	executable := Executable(source, testContext)
	assert.Equal(t, executable.Text, `import { $method } from "effeck-runtime/data-methods"; import { isSome as $method_Option_isSome } from "effect/Option"; const $methodCandidates_isSome = [["Option", $method_Option_isSome]]; `+"\n"+`import { opaque } from "./opaque.js"; export const value = $method(opaque, "isSome", [], $methodCandidates_isSome);`)
	assertParses(t, executable.Text)
}

// TestDataMethodSharedCandidates checks that shared names retain every module
// candidate, while aliases cannot bind to the user's module or helper shadows.
func TestDataMethodSharedCandidates(t *testing.T) {
	source := `import { opaque } from "./opaque.js"; const Option = 1, $method_Option_filter = 2, $methodCandidates_filter = 3; function f() { return opaque.filter((n: number): boolean => n > 0) } function g() { return opaque.filter((n: number): boolean => n > 1) }`
	checking := Project(source, testContext)
	assert.Equal(t, len(checking.Diagnostics), 0, "%v", checking.Diagnostics)
	executable := Executable(source, testContext)
	assert.Equal(t, len(executable.Diagnostics), 0, "%v", executable.Diagnostics)
	assert.Assert(t, strings.Contains(executable.Text, `filter as $method_Option_filter_1`), "%s", executable.Text)
	assert.Equal(t, strings.Count(executable.Text, `const $methodCandidates_filter_1 = `), 1)
	assert.Equal(t, strings.Count(executable.Text, `, $methodCandidates_filter_1)`), 2)
	assert.Assert(t, !strings.Contains(checking.Text, "$methodCandidates_filter_1"), "%s", checking.Text)
	for _, module := range []string{"Chunk", "HashMap", "HashSet", "Option", "Stream"} {
		assert.Assert(t, strings.Contains(executable.Text, `from "effect/`+module+`"`), "%s", executable.Text)
	}
	assert.Assert(t, !strings.Contains(executable.Text, `from "effect/Effect"`) || !strings.Contains(executable.Text, `$method_Effect_filter`), "%s", executable.Text)
	assertParses(t, executable.Text)
}

// TestDataMethodCandidateCoverage keeps generated candidates complete for the
// projector's pinned methods, including names shared across data modules.
func TestDataMethodCandidateCoverage(t *testing.T) {
	for name, modules := range dataMethodCandidates {
		for _, module := range modules {
			assert.Assert(t, dataMethods["effect/"+module][name], "%s#%s", module, name)
		}
	}
	root, err := filepath.Abs("../../../..")
	assert.NilError(t, err)
	encoded, err := json.Marshal(dataMethodCandidates)
	assert.NilError(t, err)
	script := `const candidates = ` + string(encoded) + `; const modules = new Map(); for(const [name, homes] of Object.entries(candidates)) for(const home of homes) { if(!modules.has(home)) modules.set(home,await import("effect/"+home)); if(typeof modules.get(home)[name] !== "function") throw Error(home+"#"+name+" is not an exported function") }`
	cmd := exec.Command("node", "--input-type=module", "--eval", script)
	cmd.Dir = filepath.Join(root, "packages/runtime")
	out, err := cmd.CombinedOutput()
	assert.NilError(t, err, "%s", out)
}
