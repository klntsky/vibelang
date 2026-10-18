package vibelangmapper

import (
	"fmt"
	"strconv"
	"strings"
)

// formsReader reads the canonical JSON that WriteTable writes, without
// reflection, so the table's first use costs milliseconds. Anything else in
// the input fails closed.
type formsReader struct {
	text       string
	pos        int
	err        error
	params     []Param
	signatures []Signature
}

// take consumes punctuation, recording a format error instead of guessing.
func (r *formsReader) take(c byte) {
	if r.err != nil {
		return
	}
	if r.pos >= len(r.text) || r.text[r.pos] != c {
		r.err = fmt.Errorf("forms: expected %q at byte %d", c, r.pos)
		return
	}
	r.pos++
}

// str reads a JSON string without allocating for the usual unescaped case.
func (r *formsReader) str() string {
	r.take('"')
	start := r.pos
	escaped := false
	for r.err == nil && r.pos < len(r.text) {
		c := r.text[r.pos]
		r.pos++
		if c == '"' {
			s := r.text[start : r.pos-1]
			if !escaped {
				return s
			}
			s, err := strconv.Unquote(r.text[start-1 : r.pos])
			if err != nil {
				r.err = err
			}
			return s
		}
		if c < 32 {
			r.err = fmt.Errorf("forms: control byte in string")
			break
		}
		if c == '\\' {
			escaped = true
			r.pos++
		}
	}
	if r.err == nil {
		r.err = fmt.Errorf("forms: unterminated string")
	}
	return ""
}

// object visits fields directly into their destination, avoiding intermediate JSON trees.
func (r *formsReader) object(field func(string)) {
	r.take('{')
	if r.err == nil && r.pos < len(r.text) && r.text[r.pos] == '}' {
		r.pos++
		return
	}
	for r.err == nil {
		key := r.str()
		r.take(':')
		field(key)
		if r.err != nil {
			return
		}
		if r.pos < len(r.text) && r.text[r.pos] == '}' {
			r.pos++
			return
		}
		r.take(',')
	}
}

// array visits elements in declaration order.
func (r *formsReader) array(element func()) {
	r.take('[')
	if r.err == nil && r.pos < len(r.text) && r.text[r.pos] == ']' {
		r.pos++
		return
	}
	for r.err == nil {
		element()
		if r.err != nil {
			return
		}
		if r.pos < len(r.text) && r.text[r.pos] == ']' {
			r.pos++
			return
		}
		r.take(',')
	}
}

// number reads the nonnegative integers the writer emits.
func (r *formsReader) number() int {
	start := r.pos
	for r.pos < len(r.text) && r.text[r.pos] >= '0' && r.text[r.pos] <= '9' {
		r.pos++
	}
	n, err := strconv.Atoi(r.text[start:r.pos])
	if err != nil {
		r.err = err
	}
	return n
}

// unknown rejects format extensions until their meaning is implemented here.
func (r *formsReader) unknown(key string) { r.err = fmt.Errorf("forms: unsupported field %q", key) }

// class checks every class byte, as the interchange reader does.
func (r *formsReader) class() Class {
	c, err := ParseClass(r.str())
	if err != nil {
		r.err = err
	}
	return c
}

// param reads the compact string or structured parameter representation.
func (r *formsReader) param() (p Param) {
	if r.pos < len(r.text) && r.text[r.pos] == '"' {
		p.Top = r.class()
		return
	}
	r.object(func(k string) {
		switch k {
		case "t":
			p.Top = r.class()
		case "e":
			p.Elem = r.class()
		case "a":
			p.AnyProp = r.class()
		case "p":
			p.Props = map[string]Class{}
			r.object(func(k string) { p.Props[k] = r.class() })
		default:
			r.unknown(k)
		}
	})
	return
}

// signature decodes directly into the runtime signature, with no RawMessage copies.
func (r *formsReader) signature() (s Signature) {
	r.object(func(k string) {
		switch k {
		case "n":
			n := r.str()
			s.Rest = strings.HasSuffix(n, "+")
			n = strings.TrimSuffix(n, "+")
			lo, hi, found := strings.Cut(n, "-")
			var err error
			s.Min, err = strconv.Atoi(lo)
			if err != nil {
				r.err = err
			}
			s.Max = s.Min
			if found {
				s.Max, err = strconv.Atoi(hi)
				if err != nil {
					r.err = err
				}
			}
		case "p":
			n := r.arrayLen()
			if n > len(r.params) {
				r.err = fmt.Errorf("forms: parameter storage exhausted")
				return
			}
			if n > 0 {
				s.Params = r.params[:0:n]
				r.params = r.params[n:]
			}
			r.array(func() { s.Params = append(s.Params, r.param()) })
		case "df":
			s.DataFirst = r.number() != 0
		case "dl":
			s.DataLast = r.number() != 0
		case "sf":
			s.Self = r.number() != 0
		case "r":
			s.Result = r.str()
		case "ra":
			s.Runs = r.str()
		default:
			r.unknown(k)
		}
	})
	return
}

// header reads provenance too, so the fast table preserves the complete source.
func (r *formsReader) header(h *Header) {
	r.object(func(k string) {
		switch k {
		case "generated":
			h.Notice = r.str()
		case "command":
			h.Command = r.str()
		case "rules":
			h.Rules = r.number()
		case "modules":
			h.Modules = r.number()
		case "keys":
			h.Keys = r.number()
		case "aliases":
			h.Aliases = r.number()
		case "methodModules":
			r.array(func() { h.MethodModules = append(h.MethodModules, r.str()) })
		case "inputs":
			r.array(func() {
				var in Input
				r.object(func(k string) {
					switch k {
					case "package":
						in.Package = r.str()
					case "version":
						in.Version = r.str()
					case "files":
						in.Files = r.number()
					case "sha256":
						in.SHA256 = r.str()
					default:
						r.unknown(k)
					}
				})
				h.Inputs = append(h.Inputs, in)
			})
		default:
			r.unknown(k)
		}
	})
}

// readTable parses the writer's fixed shape in one pass. Strings share
// one immutable copy of the embedded bytes for the lifetime of the table.
func readTable(data []byte) (*Table, error) {
	r := formsReader{text: string(data)}
	// Count parameter slots before decoding so all signatures share two backing
	// allocations. The scan recognizes nested objects and escaped strings.
	total := 0
	for offset := 0; offset < len(r.text); {
		i := strings.Index(r.text[offset:], `"p":[`)
		if i < 0 {
			break
		}
		r.pos = offset + i + 4
		total += r.arrayLen()
		offset = r.pos + 1
	}
	r.pos = 0
	r.params = make([]Param, total)
	r.signatures = make([]Signature, strings.Count(r.text, `"n":`))
	t := &Table{}
	r.header(&t.Header)
	t.entries = make(map[string][]Signature, t.Header.Keys)
	t.aliases = make(map[string]string, t.Header.Aliases)
	t.values = map[string]string{}
	t.modules = make([]string, 0, t.Header.Modules)
	r.take('\n')
	for r.err == nil && r.pos < len(r.text) {
		var key, alias, target, value, module string
		var sigs []Signature
		r.object(func(k string) {
			switch k {
			case "key":
				key = r.str()
			case "alias":
				alias = r.str()
			case "to":
				target = r.str()
			case "value":
				value = r.str()
			case "module":
				module = r.str()
			case "sigs":
				n := r.arrayLen()
				if n > len(r.signatures) {
					r.err = fmt.Errorf("forms: signature storage exhausted")
					return
				}
				sigs = r.signatures[:0:n]
				r.signatures = r.signatures[n:]
				r.array(func() { sigs = append(sigs, r.signature()) })
			default:
				r.unknown(k)
			}
		})
		switch {
		case key != "":
			t.entries[key] = sigs
		case alias != "":
			t.aliases[alias] = target
		case value != "":
			t.values[value] = module
		case module != "":
			t.modules = append(t.modules, module)
		default:
			r.err = fmt.Errorf("forms: empty entry")
		}
		r.take('\n')
	}
	if r.err != nil {
		return nil, r.err
	}
	return t, nil
}

// arrayLen counts immediate elements without decoding them, allowing a single
// backing allocation while preserving nested records and quoted punctuation.
func (r *formsReader) arrayLen() int {
	if r.pos >= len(r.text) || r.text[r.pos] != '[' {
		return 0
	}
	depth, count := 0, 0
	quoted := false
	for i := r.pos; i < len(r.text); i++ {
		c := r.text[i]
		if quoted {
			if c == '\\' {
				i++
			} else if c == '"' {
				quoted = false
			}
			continue
		}
		switch c {
		case '"':
			quoted = true
		case '[', '{':
			depth++
		case '}', ']':
			depth--
			if depth == 0 {
				if i > r.pos+1 {
					count++
				}
				return count
			}
		case ',':
			if depth == 1 {
				count++
			}
		}
	}
	return 0
}
