package vibelangfmt

import (
	"bytes"
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "rewrite the golden files of TestFormatGolden")

// forms holds one source per VibeLang form, written as an author might, so
// a test sees each form's TypeScript form and its round trip.
var forms = map[string]string{
	"effect types": `type Find = (id: string) => User!NotFound
type Release = (version: string) => string!!Approval
type Load = () => string!!
function hello(id: string): string!NotFound!Users {
  return getUser(id).name
}
`,
	"declarations": `/** A missing user. */
export error NotFound { id: string }
error Timeout
error Wrapped<T> {
  cause: T
  mutable attempts: number
}
service Users {
  find(id: string): User!NotFound
  region: string
}
control Approval {
  approve(request: string): boolean!!
}
schema Account {
  id: string
  nickname?: string
  role: "admin" | "member"
}
interface Cart { id: string; mutable items: Array<string>; mutable [key: string]: unknown }
`,
	"block params": `const UsersLive = Layer.effect(Users) {
  const seen = new Set<string>()
  return { find(id) { return getUser(id) }, region: "eu" }
}
function greetUser(id: string) {
  return withSpan("greetUser") {
    const user = getUser(id)
    return user.name
  }
}
it.live("finds a user") {
  assert.strictEqual(getUser("1").name, "Ada")
}
const traced = (id: string) => withSpan("traced") { return id }
const fresh = Layer.unwrap { return Layer.empty }
`,
	"match and is": `function describe(e: NotFound | Timeout) {
  return match (e) {
    when NotFound and { let id }: ` + "`no user ${id}`" + `;
    when Timeout: "retry";
  }
}
function phase(p: string) { return match (p) { when "a": 1; default: 2; } }
function failed(outcome: Exit.Exit<User, NotFound>) {
  if (outcome is Exit.Failure(let cause) && Cause.hasFails(cause)) return Cause.pretty(cause)
  return outcome is Exit.Success
}
function statement(x: number) {
  match (x) {
    when 1: log("one");
    default: log("other");
  }
}
`,
	"catch and throw": `function nameOrGuest(id: string) {
  return getUser(id).name catch "guest"
}
function nameOrReason(id: string) {
  return getUser(id).name catch (e) => match (e) {
    when NotFound: ` + "`no user ${e.id}`" + `;
  }
}
function find(users: Map<string, User>, id: string) {
  return users.get(id) ?? throw new NotFound({ id })
}
const chained = a() catch b() catch 1
const typed = (x as number) catch 0
`,
	"try clauses": `function describe(id: string) {
  try {
    return getUser(id).name
  } catch (e) {
    return match (e) {
      when NotFound: "missing";
      default: throw e;
    }
  } finally {
    log("done")
  }
}
function auto(version: string) {
  try {
    return release(version)
  } handle Approval.approve(request) {
    resume(true)
  }
}
function answered(version: string) {
  try {
    return release(version)
  } handle Approval.approve(request) {
    resume(request.length > 0)
  } catch (e) {
    return match (e) { when NotFound: "missing"; default: throw e; }
  } finally {
    log("released")
  }
}
function tracked() {
  try {
    return getUser("1")
  } finally (exit) {
    log(` + "`lookup ${exit._tag}`" + `)
  }
}
function single() {
  try { return read() } catch (e) { return e is Gone ? e.id : "other" }
}
`,
	"blocks and fibers": `function label(id: string) {
  const name = do {
    const user = getUser(id)
    if (user.name.length > 3) user.name
    else "short"
  }
  return name
}
function program() {
  provide UsersLive {
    return Users.find("1")
  }
}
function roll(): number!! {
  provide Logger.layer([]), Random = seeded {
    return Random.nextInt()
  }
}
function guarded() {
  uninterruptible {
    const user = getUser("1")
    restore {
      return { user, orders: getOrders(user.id) }
    }
  }
}
function both() {
  const user = fork getUser("1")
  fork {
    while (true) {
      log("tick")
    }
  }
  return [join user, join [user]]
}
function many() {
  const [user, orders] = await.all [getUser("1"), getOrders("1")]
  const pair = await.allSettled { user: getUser("1") }
  return await.race [user, do { sleep("1 second"); "timeout" }]
}
function logAll(ids: string[]) {
  for concurrent(8) (const id of ids) log(getUser(id).name)
  for concurrent (const id of ids) {
    log(id)
  }
}
`,
	// Superseded and retired forms still parse, so the formatter keeps them:
	// the layer declaration (D221), service class (D223), with (D172),
	// select (EK4607), and typed and repeated catch clauses (D216).
	"retired and older forms": `layer UsersLive: Users {
  const directory = Directory()
  find(id) { return directory.get(id) }
  readonly region = "eu"
}
service class Accounts {
  find(id: string): Account!NotFound
}
function f(jobs: Queue.Queue<string>) {
  with UsersLive {
    select {
      case const job = jobs.take(): run(job)
      default: idle()
    }
  }
  const fiber = fork read()
  return await fiber
}
function typed(id: string) {
  try {
    return getUser(id).name
  } catch (e: NotFound) {
    return "missing"
  } catch (e: Timeout | Offline) {
    return "slow"
  } catch (e) {
    return "other"
  } finally {
    log("done")
  }
}
`,
	"pipes and hatches": `function piped() {
  return getUser("1") |> retry(%, Schedule.recurs(2)) |> timeout(%, "1 second")
}
function modulo(n: number) { return n |> % % 2 |> (% ? 1 : 0) }
function nullish(a?: number) { return a ?? 1 |> % + 1 }
function hatches(text: string) {
  const id = sync(crypto.randomUUID())
  const parsed = try((text) => JSON.parse(text), (error) => new BadJson({ error }), text)
  return callback<number>((resume) => resume(parsed + id))
}
`,
}

// TestEncodingRoundTrips checks that decoding the TypeScript form of each
// form, unformatted, gives back the source exactly.
func TestEncodingRoundTrips(t *testing.T) {
	t.Parallel()
	for name, source := range forms {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			enc, err := encode(name, source)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(enc.text, "!NotFound") || strings.Contains(enc.text, "|>") || strings.Contains(enc.text, " catch \"") {
				t.Fatalf("VibeLang syntax left in the TypeScript form:\n%s", enc.text)
			}
			decoded, err := enc.decode(enc.text)
			if err != nil {
				t.Fatalf("%v\nTypeScript form:\n%s", err, enc.text)
			}
			if decoded != source {
				t.Fatalf("round trip changed the file:\n%s\nTypeScript form:\n%s", decoded, enc.text)
			}
		})
	}
}

// TestFormatRejects checks the files Format refuses rather than change.
func TestFormatRejects(t *testing.T) {
	t.Parallel()
	identity := func(text string) (string, error) { return text, nil }
	cases := []struct{ name, source, message string }{
		{"syntax error", "const x = (\n", "1:"},
		{"comment between a call and its block", "const v = withSpan(\"x\") /* why */ {\n  return 1\n}\n", "comment inside VibeLang syntax"},
		{"comment between try clauses", "function f() {\n  try {\n    a()\n  } handle Ask.answer(q) {\n    resume(true)\n  } // next\n  catch (e) {\n    c()\n  }\n}\n", "comment between the clauses"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			_, err := Format("x.vibe", c.source, identity)
			if err == nil || !strings.Contains(err.Error(), c.message) {
				t.Fatalf("got %v, want an error containing %q", err, c.message)
			}
		})
	}
}

// TestFormatRefusesChangedPrograms checks that a formatter that changes the
// program fails the file.
func TestFormatRefusesChangedPrograms(t *testing.T) {
	t.Parallel()
	swapped := func(text string) (string, error) { return strings.Replace(text, "a + b", "a - b", 1), nil }
	if _, err := Format("x.vibe", "const v = withSpan(\"x\") { return a + b }\n", swapped); err == nil || !strings.Contains(err.Error(), "changed the program") {
		t.Fatalf("got %v, want a changed-program error", err)
	}
}

// dprintFormatter formats TypeScript with the dprint named by VIBELANG_DPRINT
// and testdata/dprint.json, the Smithers repository's TypeScript style. The
// tests that need it skip without it.
func dprintFormatter(t *testing.T) TypeScriptFormatter {
	t.Helper()
	binary := os.Getenv("VIBELANG_DPRINT")
	if binary == "" {
		t.Skip("set VIBELANG_DPRINT to a dprint binary to run the formatter tests")
	}
	config, err := filepath.Abs(filepath.Join("testdata", "dprint.json"))
	if err != nil {
		t.Fatal(err)
	}
	return func(text string) (string, error) {
		command := exec.Command(binary, "fmt", "--config", config, "--stdin", "file.ts")
		command.Stdin = strings.NewReader(text)
		var stdout, stderr bytes.Buffer
		command.Stdout, command.Stderr = &stdout, &stderr
		if err := command.Run(); err != nil {
			return "", &formatterError{err: err, stderr: stderr.String()}
		}
		return stdout.String(), nil
	}
}

type formatterError struct {
	err    error
	stderr string
}

func (e *formatterError) Error() string { return e.err.Error() + ": " + e.stderr }

// TestFormatGolden formats each testdata/format/*.vibe file, compares the
// result with its .golden file, and checks that formatting the result again
// changes nothing.
func TestFormatGolden(t *testing.T) {
	formatter := dprintFormatter(t)
	inputs, err := filepath.Glob(filepath.Join("testdata", "format", "*.vibe"))
	if err != nil || len(inputs) == 0 {
		t.Fatalf("no golden inputs: %v", err)
	}
	for _, input := range inputs {
		t.Run(filepath.Base(input), func(t *testing.T) {
			source, err := os.ReadFile(input)
			if err != nil {
				t.Fatal(err)
			}
			got, err := Format(input, string(source), formatter)
			if err != nil {
				t.Fatal(err)
			}
			golden := strings.TrimSuffix(input, ".vibe") + ".golden"
			if *update {
				if err := os.WriteFile(golden, []byte(got), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			want, err := os.ReadFile(golden)
			if err != nil {
				t.Fatalf("%v (run with -update to write it)", err)
			}
			if got != string(want) {
				t.Fatalf("formatted:\n%s\nwant:\n%s", got, want)
			}
			again, err := Format(golden, got, formatter)
			if err != nil {
				t.Fatal(err)
			}
			if again != got {
				t.Fatalf("formatting is not idempotent; the second pass gives:\n%s", again)
			}
		})
	}
}

// TestFormatForms formats every form and checks that the result is stable
// and the same program.
func TestFormatForms(t *testing.T) {
	formatter := dprintFormatter(t)
	for name, source := range forms {
		t.Run(name, func(t *testing.T) {
			once, err := Format(name, source, formatter)
			if err != nil {
				t.Fatal(err)
			}
			twice, err := Format(name, once, formatter)
			if err != nil {
				t.Fatal(err)
			}
			if once != twice {
				t.Fatalf("not idempotent:\n%s\nthen:\n%s", once, twice)
			}
		})
	}
}

var vibeBlock = regexp.MustCompile("(?s)```vibe[^\n]*\n(.*?)```")

// TestFormatDocsCorpus formats every vibe block of the user docs that parses,
// and checks that formatting succeeds, keeps the program, and is stable.
func TestFormatDocsCorpus(t *testing.T) {
	formatter := dprintFormatter(t)
	docs := filepath.Join("..", "..", "..", "..", "docs", "user", "src")
	var pages []string
	err := filepath.WalkDir(docs, func(path string, entry os.DirEntry, err error) error {
		if err == nil && !entry.IsDir() && strings.HasSuffix(path, ".md") {
			pages = append(pages, path)
		}
		return err
	})
	if err != nil {
		t.Skipf("no user docs: %v", err)
	}
	formatted, skipped := 0, 0
	for _, page := range pages {
		text, err := os.ReadFile(page)
		if err != nil {
			t.Fatal(err)
		}
		for i, match := range vibeBlock.FindAllStringSubmatch(string(text), -1) {
			source := match[1]
			if _, err := encode(page, source); err != nil && strings.Contains(err.Error(), "vibelang format: ") && !strings.Contains(err.Error(), "internal error") && !strings.Contains(err.Error(), "comment") {
				skipped++
				continue // a block that does not parse, such as one showing a syntax error
			}
			name := filepath.Base(page) + "#" + strconv.Itoa(i)
			once, err := Format(name, source, formatter)
			if err != nil {
				t.Errorf("%s: %v\n%s", name, err, source)
				continue
			}
			twice, err := Format(name, once, formatter)
			if err != nil {
				t.Errorf("%s: second pass: %v\n%s", name, err, once)
				continue
			}
			if once != twice {
				t.Errorf("%s: not idempotent:\n%s\nthen:\n%s", name, once, twice)
			}
			formatted++
		}
	}
	t.Logf("formatted %d docs blocks from %d pages; skipped %d that do not parse", formatted, len(pages), skipped)
	if formatted < 500 {
		t.Fatalf("formatted only %d docs blocks", formatted)
	}
}
