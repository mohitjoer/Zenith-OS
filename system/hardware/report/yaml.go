// Package report renders a machine's detection result into the shapes
// `zenith hardware report` produces.
//
// Every renderer here runs the redaction sweep before returning bytes. That is
// not a wrapper around the package — it is the package's contract: a renderer
// that can return an unredacted report is a renderer that will eventually do so
// by accident, and the output of this tool is attached to public pull requests.
//
// There is no YAML dependency. The emitter below writes the subset of YAML the
// profile schema uses, which keeps this module free of external dependencies
// and — more to the point — makes the one genuinely dangerous part of emitting
// YAML explicit and testable: quoting a scalar that would otherwise change type
// when it is read back.
package report

import (
	"fmt"
	"strconv"
	"strings"
)

// scalar quoting
//
// A YAML scalar that looks like something other than a string is parsed as that
// other thing. `product_version: 1.0` parses as a float, and then compares
// unequal to the string "1.0" that the machine reports — so a correct profile
// silently fails to select its own machine.
//
// This is called out in the design record as "the single most common way a
// correct profile fails to select its own machine", which is why it gets a
// dedicated function and dedicated tests rather than being handled by a library
// whose defaults nobody has read.

// yamlBareSafe reports whether a scalar can be written without quotes.
//
// The rule is deliberately conservative: anything that is not plainly a word
// gets quotes. A quoted scalar is always correct; an unquoted one is correct
// only under a set of rules that grow with every YAML version.
func yamlBareSafe(value string) bool {
	if value == "" {
		return false
	}

	// A leading indicator character changes the meaning of the line or starts a
	// different node type.
	switch value[0] {
	case '-', '?', ':', ',', '[', ']', '{', '}', '#', '&', '*', '!', '|', '>',
		'\'', '"', '%', '@', '`', ' ':
		return false
	}

	// Trailing or leading whitespace is not preserved by an unquoted scalar.
	if value != strings.TrimSpace(value) {
		return false
	}

	// ": " and " #" introduce a mapping and a comment inside a plain scalar.
	if strings.Contains(value, ": ") || strings.Contains(value, " #") {
		return false
	}
	if strings.HasSuffix(value, ":") {
		return false
	}

	// Resolved words. "no" is a plain boolean in YAML 1.1 and a string in 1.2, and
	// a profile read by the wrong one stops matching.
	switch strings.ToLower(value) {
	case "true", "false", "yes", "no", "on", "off", "null", "~", "none":
		return false
	}

	// Anything a number could be. This includes things that only parse as
	// numbers in one base, which is why the check is not "does it parse as a
	// float" — that would let "0x10" and "1_000" through unquoted.
	if looksNumeric(value) {
		return false
	}

	// A date-shaped scalar.
	if looksLikeDate(value) {
		return false
	}

	for _, r := range value {
		if r < 0x20 || r == 0x7F {
			return false
		}
	}

	return true
}

// looksNumeric reports whether a scalar would parse as a number rather than a
// string.
func looksNumeric(value string) bool {
	if _, err := strconv.ParseFloat(value, 64); err == nil {
		return true
	}
	if _, err := strconv.ParseInt(value, 0, 64); err == nil {
		return true
	}
	// Octal, hex and binary with the YAML 0o/0x/0b prefixes, and the
	// underscore-separated forms YAML permits.
	trimmed := strings.ReplaceAll(value, "_", "")
	for _, prefix := range []string{"0x", "0X", "0o", "0O", "0b", "0B"} {
		if strings.HasPrefix(trimmed, prefix) {
			if _, err := strconv.ParseInt(trimmed[2:], 0, 64); err == nil {
				return true
			}
		}
	}
	switch strings.ToLower(value) {
	case ".inf", "-.inf", "+.inf", ".nan":
		return true
	}
	return false
}

// looksLikeDate reports whether a scalar would parse as a timestamp.
//
// The separator set includes "/" as well as the ISO forms, because the SMBIOS
// firmware date is mm/dd/yyyy and a profile carries it. Quoting a value that
// turns out not to be a timestamp costs nothing; not quoting one that is costs
// a profile that silently stops matching its own machine.
func looksLikeDate(value string) bool {
	digits, separators := 0, 0
	for _, r := range value {
		switch {
		case r >= '0' && r <= '9':
			digits++
		case r == '-' || r == '/' || r == ':' || r == 'T' || r == 't' || r == ' ':
			separators++
		default:
			return false
		}
	}
	return digits >= 6 && separators >= 2
}

// YAMLString renders a string scalar, quoting it when it needs it.
func YAMLString(value string) string {
	if yamlBareSafe(value) {
		return value
	}
	return quote(value)
}

// quote wraps a scalar in double quotes, escaping what a double-quoted YAML
// scalar requires escaped.
func quote(value string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range value {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}

// yamlWriter builds a YAML document with two-space indentation and explicit,
// deterministic key order.
//
// The order is written out rather than derived from a map, because a report that
// reorders its keys between runs is a report whose diffs are noise. The golden
// test in this package depends on byte-identical output, and so does a reviewer
// reading two reports side by side.
type yamlWriter struct {
	b strings.Builder
}

func newYAML() *yamlWriter {
	return &yamlWriter{}
}

// key writes a bare "name:" at a level, opening a nested map or a block.
//
// It deliberately writes no value and no trailing space. Writing "gpu: " and
// then nesting underneath it produces a line that is both trailing-whitespace
// and, for an intermediate level, syntactically wrong.
func (w *yamlWriter) key(level int, name string) {
	w.b.WriteString(indent(level))
	w.b.WriteString(name)
	w.b.WriteString(":\n")
}

// scalar writes "name: value", quoting the value when it must be quoted.
func (w *yamlWriter) scalar(level int, name, value string) {
	w.b.WriteString(indent(level))
	w.b.WriteString(name)
	w.b.WriteString(": ")
	w.b.WriteString(YAMLString(value))
	w.b.WriteByte('\n')
}

// raw writes "name: value" with no quoting, for values that are known to be
// plain: integers, and the fixed vocabulary words the schema defines.
func (w *yamlWriter) raw(level int, name, value string) {
	w.b.WriteString(indent(level))
	w.b.WriteString(name)
	w.b.WriteString(": ")
	w.b.WriteString(value)
	w.b.WriteByte('\n')
}

// emptyList writes "name: []" — the shape the profile schema uses for an empty
// list, which is not the same as an absent key.
func (w *yamlWriter) emptyList(level int, name string) {
	w.raw(level, name, "[]")
}

// list writes "name: [a, b, c]" on one line. The profile schema's lists are short
// and single-valued, and a one-line list is what the example in the design
// record shows.
func (w *yamlWriter) list(level int, name string, values []string) {
	if len(values) == 0 {
		w.emptyList(level, name)
		return
	}
	quoted := make([]string, 0, len(values))
	for _, value := range values {
		quoted = append(quoted, YAMLString(value))
	}
	w.raw(level, name, "["+strings.Join(quoted, ", ")+"]")
}

// blank writes an empty line, used between top-level blocks so the output reads
// as the example does.
func (w *yamlWriter) blank() {
	w.b.WriteByte('\n')
}

// Bytes returns the rendered document.
func (w *yamlWriter) Bytes() []byte { return []byte(w.b.String()) }

// indent returns a string of n two-space indents.
func indent(n int) string { return strings.Repeat("  ", n) }

// comment writes a comment line at the given level.
func (w *yamlWriter) comment(level int, text string) {
	for _, line := range strings.Split(text, "\n") {
		w.b.WriteString(indent(level))
		w.b.WriteString("#")
		if line != "" {
			w.b.WriteByte(' ')
			w.b.WriteString(line)
		}
		w.b.WriteByte('\n')
	}
}

// folded writes a folded block scalar ("name: >"), which is how the schema
// carries notes.
//
// A folded scalar is used rather than a quoted one because the design record's
// own example uses it, and because a note is prose: writing what someone tried
// is several sentences long and wrapping it in escaped quotes makes it harder to
// read and easier to corrupt.
func (w *yamlWriter) folded(level int, name, text string) {
	w.b.WriteString(indent(level))
	w.b.WriteString(name)
	w.b.WriteString(": >\n")
	lines := strings.Split(strings.TrimRight(text, "\n"), "\n")
	for _, line := range lines {
		w.b.WriteString(indent(level + 1))
		w.b.WriteString(line)
		w.b.WriteByte('\n')
	}
}

// errorf is a small helper so renderers can wrap consistently.
func errorf(format string, args ...any) error { return fmt.Errorf(format, args...) }
