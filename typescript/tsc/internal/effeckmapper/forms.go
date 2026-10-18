package effeckmapper

// The forms table (language change L1, research/typedirected.md section 6).
// The projector reads no types, so it cannot ask the checker which form a
// lambda takes (D56), whether an argument is passed by name (D147), where a
// block goes (D188), or what a value converts to (D7, D49). It asks this
// table instead. cmd/effeck-forms generates it from the declarations of the
// packages Effeck pins (effect@4.0.5, the @effect/* packages the corpus uses,
// and effeck-runtime) with the fork's own classifier, the code that makes
// those decisions in internal/effeck. scripts/check-forms-table.sh fails when
// the committed table differs from a fresh one. Never edit forms_table.jsonl.
//
// The table is keyed by callee: a module path, #, and an export path, as in
// effect/Effect#forEach or @effect/vitest#it.effect. Each key holds the
// callee's call signatures in declaration order. A signature records its
// arity and, per parameter, the Class of the declared, uninstantiated type at
// the top level, for an array element, and for each object property. Aliases
// map the other names of a module or export to the canonical key:
// effect#Effect is the module effect/Effect, prelude#forEach is
// effect/Effect#forEach (D151), prelude#wire is effeck-runtime#wire.
//
// A callee is absent when none of its parameters carries a form: a lambda or
// a call passed to it takes the defaults (forms_resolve.go), as for a callee
// the table does not cover.

import (
	"bufio"
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"
	"sync"
)

//go:embed forms_table.jsonl
var embeddedTable []byte

// Class is the set of forms a parameter position accepts, as the fork's
// classifier reads its declared type. A union of an Effect and a function
// type accepts both (D39).
type Class uint8

const (
	// ClassProgram accepts an Effect value.
	ClassProgram Class = 1 << iota
	// ClassEffectFn accepts a function that returns an Effect.
	ClassEffectFn
	// ClassPlainFn accepts a function that returns something else.
	ClassPlainFn
)

// String writes c as the table does: P, E, and F in that order, or "".
func (c Class) String() string {
	s := ""
	if c&ClassProgram != 0 {
		s += "P"
	}
	if c&ClassEffectFn != 0 {
		s += "E"
	}
	if c&ClassPlainFn != 0 {
		s += "F"
	}
	return s
}

// ParseClass reads a Class written as String writes it.
func ParseClass(s string) (Class, error) {
	var c Class
	for _, r := range s {
		switch r {
		case 'P':
			c |= ClassProgram
		case 'E':
			c |= ClassEffectFn
		case 'F':
			c |= ClassPlainFn
		default:
			return 0, fmt.Errorf("forms: bad class %q", s)
		}
	}
	return c, nil
}

// Has reports whether c accepts every form in other.
func (c Class) Has(other Class) bool { return c&other == other && other != 0 }

// Param is what one declared parameter accepts.
type Param struct {
	// Top is the class of the argument itself.
	Top Class
	// Elem is the class of an element of an array argument, as in
	// Effect.all([a, b]) (D49).
	Elem Class
	// Props is the class of each named property of an object argument, as
	// in Effect.match(self, { onSuccess, onFailure }).
	Props map[string]Class
	// AnyProp is the class of a property Props does not name: the
	// parameter type's string index signature, as in Effect.all({ a, b }).
	AnyProp Class
}

// IsZero reports whether p carries no form anywhere.
func (p Param) IsZero() bool {
	return p.Top == 0 && p.Elem == 0 && len(p.Props) == 0 && p.AnyProp == 0
}

// At returns the class at the steps below the argument: none for the
// argument itself, then an array element or an object property. Deeper
// paths carry no class.
func (p Param) At(steps []PathStep) Class {
	switch len(steps) {
	case 0:
		return p.Top
	case 1:
		if steps[0].Element {
			return p.Elem
		}
		if c, ok := p.Props[steps[0].Property]; ok {
			return c
		}
		return p.AnyProp
	}
	return 0
}

// Equal reports whether p and q record the same classes.
func (p Param) Equal(q Param) bool {
	if p.Top != q.Top || p.Elem != q.Elem || p.AnyProp != q.AnyProp || len(p.Props) != len(q.Props) {
		return false
	}
	for k, v := range p.Props {
		if w, ok := q.Props[k]; !ok || w != v {
			return false
		}
	}
	return true
}

// Signature is one call signature of a callee.
type Signature struct {
	// Min is the number of required parameters; Max the number declared.
	Min, Max int
	// Rest reports whether the last declared parameter is a rest parameter.
	Rest bool
	// Params holds each declared parameter, without trailing ones that
	// carry no form.
	Params []Param
	// DataFirst reports whether the first parameter is an Effect: the
	// data-first form of an Effect dual, which takes a block first (D188).
	DataFirst bool
	// DataLast reports whether the result is a function whose first
	// parameter is an Effect: the data-last form, which a block is applied to.
	DataLast bool
}

// Accepts reports whether s takes argc arguments.
func (s Signature) Accepts(argc int) bool {
	return argc >= s.Min && (s.Rest || argc <= s.Max)
}

// Param returns what s's parameter for argument i accepts. A rest
// parameter answers for every argument from its position on.
func (s Signature) Param(i int) Param {
	if s.Rest && i >= s.Max-1 {
		i = s.Max - 1
	}
	if i >= 0 && i < len(s.Params) {
		return s.Params[i]
	}
	return Param{}
}

// Forms is the table's answer for one callee called with argc arguments.
type Forms struct {
	// Callee is the canonical key the answer comes from.
	Callee string
	// Argc is the argument count asked for.
	Argc int
	// Candidates are the signatures that take Argc arguments, the one that
	// requires the most arguments first (a dual's data-first form once its
	// data is there, as effect's own dual(arity) decides), then in
	// declaration order.
	Candidates []Signature
	// Params holds, for each argument, its Param when every candidate
	// agrees, else the first candidate's; Agree says which.
	Params []Param
	// Agree reports, per argument, whether every candidate gives it the
	// same Param.
	Agree []bool
}

// Agreed reports whether every candidate agrees on every argument, so
// Params is the answer without looking at the arguments. When they
// disagree, the projector calls Resolver.Lambda or Resolver.Argument, which
// drop the candidates the written arguments rule out (shapeFit) and take
// the first one left; the decision then carries Ambiguous.
func (f Forms) Agreed() bool { return !slices.Contains(f.Agree, false) }

// Header is the table's first line: what generated it, from what.
type Header struct {
	// Notice is "generated; do not edit".
	Notice string `json:"generated"`
	// Command regenerates the table, from the repository root.
	Command string `json:"command"`
	// Inputs are the declaration packages the table was generated from.
	Inputs []Input `json:"inputs"`
	// Modules are the module specifiers the generator walked.
	Modules []string `json:"modules"`
	// Keys and Aliases count the table's entries.
	Keys    int `json:"keys"`
	Aliases int `json:"aliases"`
}

// Input pins one declaration package by version and by a hash of its
// declaration files.
type Input struct {
	Package string `json:"package"`
	Version string `json:"version"`
	// Files is how many declaration files the hash covers.
	Files int `json:"files"`
	// SHA256 hashes each declaration file's path relative to the package
	// and its contents, in path order.
	SHA256 string `json:"sha256"`
}

// Table is a parsed forms table.
type Table struct {
	Header  Header
	entries map[string][]Signature
	aliases map[string]string
}

// NewTable returns a table that holds entries and aliases. An alias maps a
// key, or a module path without #, to its canonical key or module path.
func NewTable(header Header, entries map[string][]Signature, aliases map[string]string) *Table {
	return &Table{Header: header, entries: entries, aliases: aliases}
}

var (
	defaultTableOnce sync.Once
	defaultTable     *Table
	defaultTableErr  error
)

// DefaultTable returns the embedded table.
func DefaultTable() (*Table, error) {
	defaultTableOnce.Do(func() {
		defaultTable, defaultTableErr = ReadTable(bytes.NewReader(embeddedTable))
	})
	return defaultTable, defaultTableErr
}

// FormsFor returns the embedded table's answer for callee called with argc
// arguments. callee is a key as the projector writes it from scope alone
// (effect#Effect.forEach, prelude#forEach, effect/Effect#forEach); aliases
// lead it to the canonical key. It reports false when the table has no
// entry for callee or none of its signatures takes argc arguments.
func FormsFor(callee string, argc int) (Forms, bool) {
	t, err := DefaultTable()
	if err != nil {
		return Forms{}, false
	}
	return t.FormsFor(callee, argc)
}

// FormsFor is the package function FormsFor over t.
func (t *Table) FormsFor(callee string, argc int) (Forms, bool) {
	key, sigs, ok := t.Lookup(callee)
	if !ok {
		return Forms{}, false
	}
	f := Forms{Callee: key, Argc: argc}
	for _, s := range sigs {
		if s.Accepts(argc) {
			f.Candidates = append(f.Candidates, s)
		}
	}
	if len(f.Candidates) == 0 {
		return Forms{}, false
	}
	slices.SortStableFunc(f.Candidates, func(a, b Signature) int { return b.Min - a.Min })
	f.Params = make([]Param, argc)
	f.Agree = make([]bool, argc)
	for i := range argc {
		f.Params[i] = f.Candidates[0].Param(i)
		f.Agree[i] = true
		for _, s := range f.Candidates[1:] {
			if !s.Param(i).Equal(f.Params[i]) {
				f.Agree[i] = false
			}
		}
	}
	return f, true
}

// Lookup returns the canonical key and the signatures the table holds for
// callee, following aliases.
func (t *Table) Lookup(callee string) (string, []Signature, bool) {
	key := callee
	for range 8 {
		if sigs, ok := t.entries[key]; ok {
			return key, sigs, true
		}
		next, ok := t.resolveAlias(key)
		if !ok || next == key {
			return "", nil, false
		}
		key = next
	}
	return "", nil, false
}

// Canonical returns the canonical key for callee after aliases, whether or
// not the table has an entry for it.
func (t *Table) Canonical(callee string) string {
	key := callee
	for range 8 {
		next, ok := t.resolveAlias(key)
		if !ok || next == key {
			return key
		}
		key = next
	}
	return key
}

// resolveAlias rewrites the longest prefix of key that names an alias:
// module#a.b.c tries module#a.b.c, module#a.b, module#a, then module.
func (t *Table) resolveAlias(key string) (string, bool) {
	hash := strings.IndexByte(key, '#')
	if hash < 0 {
		return "", false
	}
	module, path := key[:hash], key[hash+1:]
	var parts []string
	if path != "" {
		parts = strings.Split(path, ".")
	}
	for i := len(parts); i >= 0; i-- {
		name := module
		if i > 0 {
			name = module + "#" + strings.Join(parts[:i], ".")
		}
		target, ok := t.aliases[name]
		if !ok {
			continue
		}
		rest := strings.Join(parts[i:], ".")
		switch {
		case !strings.Contains(target, "#"): // a module
			return target + "#" + rest, true
		case rest == "":
			return target, true
		default:
			return target + "." + rest, true
		}
	}
	return "", false
}

// Keys returns the table's keys, sorted.
func (t *Table) Keys() []string {
	keys := make([]string, 0, len(t.entries))
	for k := range t.entries {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

// Signatures returns the signatures under the canonical key, or nil.
func (t *Table) Signatures(key string) []Signature { return t.entries[key] }

// Aliases returns the table's aliases.
func (t *Table) Aliases() map[string]string { return t.aliases }

// The file format: JSON lines. The first line is the Header. Then one line
// per alias, {"alias":name,"to":target}, sorted by name, and one line per
// key, {"key":key,"sigs":[signature...]}, sorted by key. A signature is
// {"n":arity,"p":[param...],"df":1,"dl":1}: arity "2" or "1-3", with "+"
// for a rest parameter ("1-2+"); a param is "" or a class string when only
// its top level has a class, else {"t":top,"e":elem,"p":{name:class},"a":any}.

type aliasLine struct {
	Alias string `json:"alias"`
	To    string `json:"to"`
}

type keyLine struct {
	Key  string          `json:"key"`
	Sigs []signatureJSON `json:"sigs"`
}

type signatureJSON struct {
	N         string            `json:"n"`
	P         []json.RawMessage `json:"p,omitempty"`
	DataFirst int               `json:"df,omitempty"`
	DataLast  int               `json:"dl,omitempty"`
}

type paramJSON struct {
	T string            `json:"t,omitempty"`
	E string            `json:"e,omitempty"`
	P map[string]string `json:"p,omitempty"`
	A string            `json:"a,omitempty"`
}

// WriteTable writes header, aliases, and entries in the file format,
// sorted, so the same inputs always give the same bytes.
func WriteTable(w io.Writer, header Header, entries map[string][]Signature, aliases map[string]string) error {
	header.Keys, header.Aliases = len(entries), len(aliases)
	bw := bufio.NewWriter(w)
	line := func(v any) error {
		data, err := marshalLine(v)
		if err != nil {
			return err
		}
		bw.Write(data)
		return bw.WriteByte('\n')
	}
	if err := line(header); err != nil {
		return err
	}
	names := make([]string, 0, len(aliases))
	for name := range aliases {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		if err := line(aliasLine{Alias: name, To: aliases[name]}); err != nil {
			return err
		}
	}
	keys := make([]string, 0, len(entries))
	for k := range entries {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	for _, k := range keys {
		kl := keyLine{Key: k}
		for _, s := range entries[k] {
			kl.Sigs = append(kl.Sigs, encodeSignature(s))
		}
		if err := line(kl); err != nil {
			return err
		}
	}
	return bw.Flush()
}

// marshalLine is json.Marshal without HTML escaping, so <, >, and & stay
// readable in keys.
func marshalLine(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}

func encodeSignature(s Signature) signatureJSON {
	n := strconv.Itoa(s.Min)
	if s.Max != s.Min {
		n += "-" + strconv.Itoa(s.Max)
	}
	if s.Rest {
		n += "+"
	}
	out := signatureJSON{N: n}
	params := s.Params
	for len(params) > 0 && params[len(params)-1].IsZero() {
		params = params[:len(params)-1]
	}
	for _, p := range params {
		var raw []byte
		if p.Elem == 0 && len(p.Props) == 0 && p.AnyProp == 0 {
			raw, _ = json.Marshal(p.Top.String())
		} else {
			pj := paramJSON{T: p.Top.String(), E: p.Elem.String(), A: p.AnyProp.String()}
			if len(p.Props) > 0 {
				pj.P = map[string]string{}
				for k, v := range p.Props {
					pj.P[k] = v.String()
				}
			}
			raw, _ = marshalLine(pj) // encoding/json sorts map keys
		}
		out.P = append(out.P, raw)
	}
	if s.DataFirst {
		out.DataFirst = 1
	}
	if s.DataLast {
		out.DataLast = 1
	}
	return out
}

// ReadTable parses a table in the file format.
func ReadTable(r io.Reader) (*Table, error) {
	t := &Table{entries: map[string][]Signature{}, aliases: map[string]string{}}
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 1<<16), 1<<24)
	first := true
	for lineNo := 1; sc.Scan(); lineNo++ {
		data := sc.Bytes()
		if len(bytes.TrimSpace(data)) == 0 {
			continue
		}
		if first {
			first = false
			if err := json.Unmarshal(data, &t.Header); err != nil {
				return nil, fmt.Errorf("forms: line %d: header: %w", lineNo, err)
			}
			continue
		}
		var probe struct {
			Alias *string `json:"alias"`
		}
		if err := json.Unmarshal(data, &probe); err != nil {
			return nil, fmt.Errorf("forms: line %d: %w", lineNo, err)
		}
		if probe.Alias != nil {
			var a aliasLine
			if err := json.Unmarshal(data, &a); err != nil {
				return nil, fmt.Errorf("forms: line %d: %w", lineNo, err)
			}
			t.aliases[a.Alias] = a.To
			continue
		}
		var k keyLine
		if err := json.Unmarshal(data, &k); err != nil {
			return nil, fmt.Errorf("forms: line %d: %w", lineNo, err)
		}
		sigs := make([]Signature, 0, len(k.Sigs))
		for _, sj := range k.Sigs {
			s, err := decodeSignature(sj)
			if err != nil {
				return nil, fmt.Errorf("forms: line %d: %s: %w", lineNo, k.Key, err)
			}
			sigs = append(sigs, s)
		}
		t.entries[k.Key] = sigs
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return t, nil
}

func decodeSignature(sj signatureJSON) (Signature, error) {
	var s Signature
	n := sj.N
	if strings.HasSuffix(n, "+") {
		s.Rest = true
		n = n[:len(n)-1]
	}
	lo, hi, found := strings.Cut(n, "-")
	var err error
	if s.Min, err = strconv.Atoi(lo); err != nil {
		return s, fmt.Errorf("bad arity %q", sj.N)
	}
	s.Max = s.Min
	if found {
		if s.Max, err = strconv.Atoi(hi); err != nil {
			return s, fmt.Errorf("bad arity %q", sj.N)
		}
	}
	for _, raw := range sj.P {
		var p Param
		var str string
		if json.Unmarshal(raw, &str) == nil {
			if p.Top, err = ParseClass(str); err != nil {
				return s, err
			}
		} else {
			var pj paramJSON
			if err := json.Unmarshal(raw, &pj); err != nil {
				return s, err
			}
			if p.Top, err = ParseClass(pj.T); err != nil {
				return s, err
			}
			if p.Elem, err = ParseClass(pj.E); err != nil {
				return s, err
			}
			if p.AnyProp, err = ParseClass(pj.A); err != nil {
				return s, err
			}
			for k, v := range pj.P {
				c, err := ParseClass(v)
				if err != nil {
					return s, err
				}
				if p.Props == nil {
					p.Props = map[string]Class{}
				}
				p.Props[k] = c
			}
		}
		s.Params = append(s.Params, p)
	}
	s.DataFirst = sj.DataFirst != 0
	s.DataLast = sj.DataLast != 0
	return s, nil
}
