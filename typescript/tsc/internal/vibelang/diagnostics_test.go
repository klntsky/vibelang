package vibelang

import (
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/microsoft/TypeScript/tsc/internal/core"
	"github.com/microsoft/TypeScript/tsc/internal/diagnostics"
)

func TestCategory(t *testing.T) {
	t.Parallel()
	with := func(codes map[string]string) *core.CompilerOptions {
		return &core.CompilerOptions{VibeLang: &core.VibeLangOptions{Diagnostics: codes}}
	}
	tests := []struct {
		name    string
		message *Message
		options *core.CompilerOptions
		want    diagnostics.Category
		on      bool
	}{
		{"EK9100 is a migration warning", EK9100, nil, diagnostics.CategoryWarning, true},
		{"EK9100 can be an error", EK9100, with(map[string]string{"EK9100": "error"}), diagnostics.CategoryError, true},
		{"EK9100 can be a warning", EK9100, with(map[string]string{"EK9100": "warning"}), diagnostics.CategoryWarning, true},
		{"EK9100 can be off", EK9100, with(map[string]string{"EK9100": "off"}), 0, false},
		{"EK4406 can be an error", EK4406, with(map[string]string{"EK4406": "error"}), diagnostics.CategoryError, true},
		{"EK4406 can be a warning", EK4406, with(map[string]string{"EK4406": "warning"}), diagnostics.CategoryWarning, true},
		{"EK4406 is a migration warning", EK4406, nil, diagnostics.CategoryWarning, true},
		{"EK2004 is a warning by default", EK2004, nil, diagnostics.CategoryWarning, true},
		{"EK4403 is an error by default", EK4403, &core.CompilerOptions{}, diagnostics.CategoryError, true},
		{"EK2004 can be an error", EK2004, with(map[string]string{"EK2004": "error"}), diagnostics.CategoryError, true},
		{"EK4403 can be a warning", EK4403, with(map[string]string{"EK4403": "warning"}), diagnostics.CategoryWarning, true},
		{"EK4403 can be off", EK4403, with(map[string]string{"EK4403": "off"}), 0, false},
		{"other codes stay errors", EK2001, with(map[string]string{"EK2001": "off"}), diagnostics.CategoryError, true},
		{"EK4801 is an error by default", EK4801, nil, diagnostics.CategoryError, true},
		{"EK4814 is a warning by default", EK4814, nil, diagnostics.CategoryWarning, true},
		{"EK4814 can be an error", EK4814, with(map[string]string{"EK4814": "error"}), diagnostics.CategoryError, true},
		{"EK4818 is off by default", EK4818, nil, 0, false},
		{"EK4818 can be a warning", EK4818, with(map[string]string{"EK4818": "warning"}), diagnostics.CategoryWarning, true},
		{"EK4813 stays an error", EK4813, with(map[string]string{"EK4813": "off"}), diagnostics.CategoryError, true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got, on := category(test.options, test.message)
			if got != test.want || on != test.on {
				t.Errorf("category(%s) = %v, %v; want %v, %v", test.message, got, on, test.want, test.on)
			}
		})
	}
}

// TestRegistryMatchesSpec checks that the registry lists exactly the codes in
// the table of Chapter 28, plus the projector migration diagnostics.
func TestRegistryMatchesSpec(t *testing.T) {
	t.Parallel()
	spec, err := os.ReadFile("../../../../docs/design/src/428-part4-ch28-tooling.md")
	if err != nil {
		t.Fatal(err)
	}
	want := []int32{9101} // The LS-only projector suggestion has no Chapter 28 row yet.
	for _, m := range regexp.MustCompile(`(?m)^\| EK(\d{4}) \|`).FindAllStringSubmatch(string(spec), -1) {
		code, _ := strconv.Atoi(m[1])
		want = append(want, int32(code))
	}
	var got []int32
	for code := range registry {
		got = append(got, code)
	}
	slices.Sort(want)
	slices.Sort(got)
	if len(want) == 0 || !slices.Equal(got, want) {
		t.Errorf("registry codes %v differ from Chapter 28's %v", got, want)
	}
}

func TestDirectiveCategory(t *testing.T) {
	t.Parallel()
	text := strings.Join([]string{
		"a()", // 0
		"// @vibelang-diagnostics-next-line EK4801:off",
		"b()", // 2: off for the next line only
		"c()", // 3
		"/* @vibelang-diagnostics EK4801:warning */",
		"d()", // 5: a warning from here on
		`const s = "@vibelang-diagnostics EK4801:off"`,
		"e()", // 7: a string is not a comment
		"// @vibelang-diagnostics *:off EK4818:warning",
		"f()", // 9: * turns EK4801 off, and EK4818 named on its own is a warning
		"// @vibelang-diagnostics EK4801:error",
		"g()", // 11: a later comment overrides *
	}, "\n")
	at := func(line int) int {
		return len(strings.Join(strings.Split(text, "\n")[:line], "\n")) + core.IfElse(line > 0, 1, 0)
	}
	tests := []struct {
		line int
		m    *Message
		want diagnostics.Category
		on   bool
	}{
		{0, EK4801, diagnostics.CategoryError, true},
		{2, EK4801, 0, false},
		{3, EK4801, diagnostics.CategoryError, true},
		{5, EK4801, diagnostics.CategoryWarning, true},
		{7, EK4801, diagnostics.CategoryWarning, true},
		{9, EK4801, 0, false},
		{9, EK4818, diagnostics.CategoryWarning, true},
		{11, EK4801, diagnostics.CategoryError, true},
		{11, EK4818, diagnostics.CategoryWarning, true},
	}
	for _, test := range tests {
		category, on := category(nil, test.m)
		got, gotOn := directiveCategoryIn(text, at(test.line), test.m, category, on)
		if got != test.want || gotOn != test.on {
			t.Errorf("line %d, %s: got %v, %v; want %v, %v", test.line, test.m, got, gotOn, test.want, test.on)
		}
	}
}
