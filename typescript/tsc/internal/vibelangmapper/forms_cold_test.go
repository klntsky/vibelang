package vibelangmapper

import (
	"os"
	"testing"
)

// BenchmarkFormsCold measures initialization in a fresh test process; run with -benchtime=1x -count=1.
func BenchmarkFormsCold(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		if _, err := DefaultTable(); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkFormsProjectCold includes the first table initialization in a real corpus projection.
func BenchmarkFormsProjectCold(b *testing.B) {
	text, err := os.ReadFile("../../../vibelang/cases/producers/main.vibe")
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		Project(string(text), FileContext{FileName: "/producers/main.vibe", ModuleKey: "producers/main"})
	}
}
