package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// TestBuildChangesConfigIdentity: TypeScript knows a mapper's code only by
// its package name and version, and vibelang-ts-mapper@0.0.1 names every
// VibeLang build. openProject's configIdentity is the one identity the
// mapper controls, so it must change with the build; otherwise tsc --build
// and --incremental keep declarations an older projector wrote.
func TestBuildChangesConfigIdentity(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Minute)
	defer cancel()
	dir := t.TempDir()
	project := t.TempDir()
	if err := os.WriteFile(filepath.Join(project, "package.json"), []byte(`{"name":"app"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	build := func(name string, flags ...string) string {
		binary := filepath.Join(dir, name)
		args := append(append([]string{"build", "-trimpath"}, flags...), "-o", binary, ".")
		if output, err := exec.CommandContext(ctx, "go", args...).CombinedOutput(); err != nil {
			t.Fatalf("go %s: %v\n%s", strings.Join(args, " "), err, output)
		}
		return binary
	}
	identity := func(binary string) string {
		command := exec.CommandContext(ctx, binary)
		stdin, err := command.StdinPipe()
		if err != nil {
			t.Fatal(err)
		}
		stdout, err := command.StdoutPipe()
		if err != nil {
			t.Fatal(err)
		}
		if err := command.Start(); err != nil {
			t.Fatal(err)
		}
		defer func() {
			stdin.Close()
			command.Wait()
		}()
		for _, message := range []string{
			`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"positionEncodings":["utf-8"]}}`,
			fmt.Sprintf(`{"jsonrpc":"2.0","id":2,"method":"openProject","params":{"configFileName":%q,"projectHandle":"p","compilerOptions":{}}}`, filepath.Join(project, "tsconfig.json")),
		} {
			fmt.Fprintf(stdin, "Content-Length: %d\r\n\r\n%s", len(message), message)
		}
		reader := bufio.NewReader(stdout)
		for {
			length := 0
			for {
				line, err := reader.ReadString('\n')
				if err != nil {
					t.Fatalf("read from %s: %v", binary, err)
				}
				line = strings.TrimSpace(line)
				if line == "" {
					break
				}
				if value, ok := strings.CutPrefix(line, "Content-Length: "); ok {
					length, _ = strconv.Atoi(value)
				}
			}
			body := make([]byte, length)
			if _, err := io.ReadFull(reader, body); err != nil {
				t.Fatal(err)
			}
			var response struct {
				ID     int `json:"id"`
				Result struct {
					ConfigIdentity string `json:"configIdentity"`
				} `json:"result"`
			}
			if err := json.Unmarshal(body, &response); err != nil {
				t.Fatalf("%v: %s", err, body)
			}
			if response.ID == 2 {
				return response.Result.ConfigIdentity
			}
		}
	}
	plain := build("plain")
	stripped := build("stripped", "-ldflags=-s -w")
	first, again, other := identity(plain), identity(plain), identity(stripped)
	if first == "" || first != again {
		t.Fatalf("one build reported configIdentity %q, then %q", first, again)
	}
	if first == other {
		t.Fatalf("two builds share configIdentity %q", first)
	}
}
