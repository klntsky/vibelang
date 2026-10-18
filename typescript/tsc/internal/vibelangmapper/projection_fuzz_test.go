package vibelangmapper

import (
	"reflect"
	"strings"
	"testing"

	"github.com/microsoft/TypeScript/tsc/internal/core"
	"github.com/microsoft/TypeScript/tsc/internal/spanmap"
)

func TestMalformedRegionProjection(t *testing.T) {
	const source = "0000000000000000000000000000000000000000000(A0{0try {000000000000000(00}(}"
	checking := Project(source, testContext)
	if checking.Text != source {
		t.Fatalf("invalid edits must preserve the original text: %q", checking.Text)
	}
	executable := Executable(source, testContext)
	if executable.Text != "" {
		t.Fatalf("invalid edits must not emit executable text: %s", executable.Text)
	}
	for _, projection := range []Projection{checking, executable} {
		if len(projection.Diagnostics) == 0 {
			t.Fatal("invalid edit range was not diagnosed")
		}
		diagnostic := projection.Diagnostics[len(projection.Diagnostics)-1]
		if diagnostic.Code != CodeUnprojectedForm || diagnostic.Start < 0 || diagnostic.Start > len(source) || !strings.Contains(diagnostic.Message, "invalid or overlapping edits") {
			t.Fatalf("unexpected diagnostic: %+v", diagnostic)
		}
		assertValidMap(t, projection, source)
	}
}

func TestRegionJumpWithoutTarget(t *testing.T) {
	for _, statement := range []string{"break", "continue", "break missing", "continue missing"} {
		t.Run(statement, func(t *testing.T) {
			source := "function f() { try { " + statement + "; } catch {} }"
			for _, projection := range []Projection{Project(source, testContext), Executable(source, testContext)} {
				// Keep the invalid jump in the generated function so TypeScript
				// can diagnose its missing target during an incomplete edit.
				if !strings.Contains(projection.Text, statement) {
					t.Fatalf("projection lost the unresolved jump: %s", projection.Text)
				}
				assertValidMap(t, projection, source)
			}
		})
	}
}

func TestQualifiedControlProjection(t *testing.T) {
	const source = "function f() { try { return API.Ask.get() } handle API.Ask.get() { resume(true) } }"
	for _, projection := range []Projection{Project(source, testContext), Executable(source, testContext)} {
		if !strings.Contains(projection.Text, "API.Ask") {
			t.Fatalf("projection lost the qualified control: %s", projection.Text)
		}
		assertValidMap(t, projection, source)
	}
}

// FuzzProjectionSpans checks the contracts TypeScript's cache and editor
// depend on: identical source/context has identical projections, diagnostics
// stay inside the original text, and verbatim spans map to their exact source.
// Include incomplete and invalid edits, because editors project these too.
func FuzzProjectionSpans(f *testing.F) {
	for _, tc := range projectionCases {
		f.Add(tc.source)
	}
	for _, source := range []string{"", "\x00", "service", "function f( {", "// 😀\u2028export const café = 1", "service S { read(): #Bad!0 }"} {
		f.Add(source)
	}
	f.Fuzz(func(t *testing.T, source string) {
		if len(source) > 16*1024 {
			t.Skip()
		}
		for name, project := range map[string]func(string, FileContext) Projection{"checking": Project, "executable": Executable} {
			projection := project(source, testContext)
			if again := project(source, testContext); !reflect.DeepEqual(projection, again) {
				t.Fatalf("%s projection is not deterministic for %q", name, source)
			}
			mapping := spanmap.New(projection.Segments)
			if err := mapping.Validate(projection.Text, source); err != nil {
				t.Fatalf("%s: invalid editor mapping for %q: %v", name, source, err)
			}
			for _, segment := range projection.Segments {
				if segment.Kind != spanmap.KindVerbatim {
					continue
				}
				virtual := core.NewTextRange(int(segment.VirtualStart), int(segment.VirtualEnd))
				original, fidelity := mapping.VirtualToOriginalSpan(virtual)
				if !fidelity.IsExact() || original.Pos() != int(segment.OriginalStart) || original.End() != int(segment.OriginalEnd) {
					t.Fatalf("%s: verbatim span %+v maps to %v with fidelity %v", name, segment, original, fidelity)
				}
			}
			for _, diagnostic := range projection.Diagnostics {
				if diagnostic.Start < 0 || diagnostic.Length < 0 || diagnostic.Start+diagnostic.Length > len(source) {
					t.Fatalf("%s: diagnostic outside source (%d bytes): %+v", name, len(source), diagnostic)
				}
			}
		}
	})
}
