package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// captureCommand uses files rather than pipes so a full projection cannot
// block on the stdout pipe buffer. These tests must not run in parallel.
func captureCommand(t *testing.T, command func() int) (int, string, string) {
	t.Helper()
	out, err := os.CreateTemp(t.TempDir(), "stdout")
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()
	errors, err := os.CreateTemp(t.TempDir(), "stderr")
	if err != nil {
		t.Fatal(err)
	}
	defer errors.Close()
	previousOut, previousErr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = out, errors
	defer func() { os.Stdout, os.Stderr = previousOut, previousErr }()
	status := command()
	read := func(file *os.File) string {
		data, err := os.ReadFile(file.Name())
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	return status, read(out), read(errors)
}

func sourceFile(t *testing.T, source string) string {
	t.Helper()
	name := filepath.Join(t.TempDir(), "main.vibe")
	if err := os.WriteFile(name, []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	return name
}

func TestPrintProjection(t *testing.T) {
	file := sourceFile(t, `export function answer(): number { return 42 }`)
	status, out, errors := captureCommand(t, func() int { return printProjection(file) })
	if status != 0 || errors != "" {
		t.Fatalf("status %d, stderr %s", status, errors)
	}
	for _, want := range []string{"export function answer(): number { return 42 }", "// --- segments", "// ["} {
		if !strings.Contains(out, want) {
			t.Errorf("projection missing %q: %s", want, out)
		}
	}
}

func TestEmitStdout(t *testing.T) {
	file := sourceFile(t, `export function answer(): number { return 42 }`)
	status, out, errors := captureCommand(t, func() int { return emit(file, "") })
	if status != 0 || errors != "" {
		t.Fatalf("status %d, stderr %s", status, errors)
	}
	if !strings.Contains(out, "export function answer()") || !strings.Contains(out, "return 42;") || strings.Contains(out, ": number") || strings.Contains(out, "sourceMappingURL") {
		t.Fatalf("unexpected JavaScript: %s", out)
	}
}

func TestEmitWritesJavaScriptAndSourceMap(t *testing.T) {
	const source = `export function answer(): number { return 42 }`
	file := sourceFile(t, source)
	output := filepath.Join(t.TempDir(), "nested", "answer.js")
	status, stdout, stderr := captureCommand(t, func() int { return emit(file, output) })
	if status != 0 || stdout != "" || stderr != "" {
		t.Fatalf("status %d, stdout %s, stderr %s", status, stdout, stderr)
	}
	js, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(js), "return 42;") || !strings.HasSuffix(string(js), "//# sourceMappingURL=answer.js.map\n") {
		t.Fatalf("unexpected JS: %s", js)
	}
	data, err := os.ReadFile(output + ".map")
	if err != nil {
		t.Fatal(err)
	}
	var mapping struct {
		Version        int      `json:"version"`
		Sources        []string `json:"sources"`
		SourcesContent []string `json:"sourcesContent"`
		Mappings       string   `json:"mappings"`
	}
	if err := json.Unmarshal(data, &mapping); err != nil {
		t.Fatal(err)
	}
	if mapping.Version != 3 || len(mapping.Sources) != 1 || !strings.HasSuffix(mapping.Sources[0], "main.vibe") || len(mapping.SourcesContent) != 1 || mapping.SourcesContent[0] != source || mapping.Mappings == "" {
		t.Fatalf("invalid original-source mapping: %s", data)
	}
}

func TestEmitReadAndWriteErrors(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing.vibe")
	for _, command := range []func() int{func() int { return emit(missing, "") }, func() int { return printProjection(missing) }} {
		status, out, errors := captureCommand(t, command)
		if status != 1 || out != "" || !strings.Contains(errors, "vibelang-mapper:") || !strings.Contains(errors, "missing.vibe") {
			t.Fatalf("status %d, stdout %s, stderr %s", status, out, errors)
		}
	}
	file := sourceFile(t, `export const answer = 42`)
	root := t.TempDir()
	blocker := filepath.Join(root, "file")
	if err := os.WriteFile(blocker, nil, 0600); err != nil {
		t.Fatal(err)
	}
	for _, output := range []string{filepath.Join(blocker, "answer.js"), root, filepath.Join(root, "answer.js")} {
		if strings.HasSuffix(output, "answer.js") && filepath.Dir(output) == root {
			if err := os.Mkdir(output+".map", 0700); err != nil {
				t.Fatal(err)
			}
		}
		status, out, errors := captureCommand(t, func() int { return emit(file, output) })
		if status != 1 || out != "" || !strings.Contains(errors, "vibelang-mapper:") {
			t.Fatalf("%s: status %d, stdout %s, stderr %s", output, status, out, errors)
		}
	}
}

func TestLineAndColumnASCII(t *testing.T) {
	for _, tc := range []struct {
		text              string
		pos, line, column int
	}{
		{"", 0, 1, 1}, {"a\nbc", -3, 1, 1}, {"a\nbc", 2, 2, 1}, {"a\nbc", 3, 2, 2}, {"a\nbc", 99, 2, 3}, {"a\r\nb", 3, 2, 1},
	} {
		line, column := lineAndColumn(tc.text, tc.pos)
		if line != tc.line || column != tc.column {
			t.Errorf("%q at %d = (%d,%d), want (%d,%d)", tc.text, tc.pos, line, column, tc.line, tc.column)
		}
	}
}
