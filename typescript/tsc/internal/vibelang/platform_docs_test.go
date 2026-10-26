package vibelang

import (
	"cmp"
	"os"
	"regexp"
	"strings"
	"testing"
)

// The platform introduction's table says what EK4402 and EK4403 say: each
// call's Effect replacement, the module to import it from, and the layer
// that provides it.
func TestPlatformDocsTable(t *testing.T) {
	page, err := os.ReadFile("../../../../docs/user/src/platform/105-introduction.md")
	if err != nil {
		t.Fatal(err)
	}
	reads := map[string]string{"Date.now": "DateConstructor.now", "performance.now": "Performance.now", "Math.random": "Math.random", "crypto.randomUUID": "Crypto.randomUUID"}
	code := regexp.MustCompile("^`([^`]*)`$")
	rows := 0
	for _, line := range strings.Split(string(page), "\n") {
		cells := strings.Split(strings.Trim(line, "| "), " | ")
		if len(cells) != 4 || !code.MatchString(cells[0]) {
			continue
		}
		rows++
		call := code.FindStringSubmatch(cells[0])[1]
		effect, from, layer := strings.Trim(cells[1], "`"), strings.Trim(cells[2], "`"), strings.Trim(cells[3], "`")
		var s *platformService
		if key, ok := reads[call]; ok {
			read := clockReads[key]
			s = platformServices[read.service]
			if !strings.HasPrefix(read.use, effect) {
				t.Errorf("%s: the page says %s, EK4403 says %s", call, effect, read.use)
			}
		} else {
			root, member, _ := strings.Cut(call, ".")
			ref := platformRef{name: root}
			if service, ok := platformGlobals[root]; ok {
				ref.replacement = service
			} else if service, ok := platformModuleOf(root); ok {
				ref.replacement, ref.module = service, root
			} else {
				t.Errorf("%s is no platform API", call)
				continue
			}
			var members []string
			if member != "" {
				members = strings.Split(member, ".")
			}
			api := platformAPIOf(ref, members)
			s = platformServices[api.service]
			if s == nil {
				t.Errorf("%s: EK4402 names no service", call)
				continue
			}
			if example := cmp.Or(api.example, s.example); example != effect {
				t.Errorf("%s: the page says %s, EK4402 says %s", call, effect, example)
			}
		}
		if s == nil || s.imports == "" {
			if from != "the prelude" || layer != "none" {
				t.Errorf("%s: the page says %s and %s for a prelude name", call, from, layer)
			}
			continue
		}
		if !strings.Contains(s.imports, `from "`+from+`"`) {
			t.Errorf("%s: the page imports from %s, EK4402 says %s", call, from, s.imports)
		}
		if !strings.Contains(s.provider, layer) {
			t.Errorf("%s: the page provides %s, EK4402 says %s", call, layer, s.provider)
		}
	}
	if rows < 10 {
		t.Fatalf("found %d rows in the table of replacements", rows)
	}
}
