// Package paradox implements a parser for Paradox Interactive localization
// files. These files use a .yml extension but are NOT YAML: values are always
// double-quoted, keys carry a numeric version suffix, and the format tokens
// embedded in values (#bold, £icon£) collide with YAML comment syntax. A
// standard YAML parser mangles them, so we scan them by hand.
package paradox

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

// BOM is the UTF-8 byte order mark. EU5 silently ignores localization files
// that lack it, so every file we write must start with these three bytes.
const BOM = "\uFEFF"

// Entry is a single localization key/value pair.
type Entry struct {
	Key string
	// Version is the number after the colon (`key:0 "..."`). -1 when the
	// source omitted it entirely (`key: "..."`), which EU5 also accepts.
	Version int
	// Value is the unquoted string with \" and \\ escapes resolved.
	Value string
	// Line is the 1-based line number the entry was parsed from.
	Line int
	// Malformed marks an entry recovered from a source defect, currently
	// only a value whose closing quote is missing. IV2 ships a handful of
	// these; dropping the keys would silently lose translatable text, so we
	// take the rest of the line as the value and flag it instead.
	Malformed bool
}

// File is one parsed localization file.
type File struct {
	// Language is the header without the trailing colon, e.g. "l_english".
	Language string
	Entries  []Entry
	// HadBOM records whether the file started with a UTF-8 BOM.
	HadBOM bool
}

// Lookup returns the first entry with the given key.
func (f *File) Lookup(key string) (Entry, bool) {
	for _, e := range f.Entries {
		if e.Key == key {
			return e, true
		}
	}
	return Entry{}, false
}

// ParseError describes a line the parser could not make sense of.
type ParseError struct {
	Line int
	Text string
	Msg  string
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("line %d: %s: %q", e.Line, e.Msg, e.Text)
}

// Parse reads a Paradox localization file. It is deliberately permissive about
// whitespace and blank lines, and it collects rather than aborts on malformed
// lines: a single bad line in a 4000-line file should not cost us the file.
func Parse(r io.Reader) (*File, []error) {
	f := &File{}
	var errs []error

	sc := bufio.NewScanner(r)
	// Some Paradox values (event descriptions) are long; the 64KiB default is
	// not always enough.
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)

	lineNo := 0
	for sc.Scan() {
		lineNo++
		line := sc.Text()

		if lineNo == 1 && strings.HasPrefix(line, BOM) {
			f.HadBOM = true
			line = strings.TrimPrefix(line, BOM)
		}

		line = stripComment(line)
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Language header: `l_english:` on its own line.
		if lang, ok := parseHeader(line); ok {
			f.Language = lang
			continue
		}

		e, err := parseEntry(line, lineNo)
		if err != nil {
			errs = append(errs, err)
			if !e.Malformed {
				continue
			}
		}
		f.Entries = append(f.Entries, e)
	}
	if err := sc.Err(); err != nil {
		errs = append(errs, err)
	}
	return f, errs
}

// ParseString is a convenience wrapper around Parse.
func ParseString(s string) (*File, []error) {
	return Parse(strings.NewReader(s))
}

func parseHeader(line string) (string, bool) {
	if !strings.HasPrefix(line, "l_") || !strings.HasSuffix(line, ":") {
		return "", false
	}
	name := strings.TrimSuffix(line, ":")
	if strings.ContainsAny(name, " \t\"") {
		return "", false
	}
	return name, true
}

// stripComment removes a trailing `#` comment, tracking quote state so that
// format tokens inside values (`"#bold gold#!"`) survive.
func stripComment(line string) string {
	inQuotes := false
	for i := 0; i < len(line); i++ {
		switch line[i] {
		case '\\':
			if inQuotes {
				i++ // skip the escaped byte
			}
		case '"':
			inQuotes = !inQuotes
		case '#':
			if !inQuotes {
				return line[:i]
			}
		}
	}
	return line
}

// parseEntry parses `key:0 "value"`. The version number and the space after it
// are both optional.
func parseEntry(line string, lineNo int) (Entry, error) {
	colon := strings.IndexByte(line, ':')
	if colon < 0 {
		return Entry{}, &ParseError{Line: lineNo, Text: line, Msg: "no key separator"}
	}

	key := strings.TrimSpace(line[:colon])
	if key == "" {
		return Entry{}, &ParseError{Line: lineNo, Text: line, Msg: "empty key"}
	}

	rest := line[colon+1:]

	// Version digits run from the colon up to the first non-digit.
	version := -1
	i := 0
	for i < len(rest) && rest[i] >= '0' && rest[i] <= '9' {
		i++
	}
	if i > 0 {
		version = 0
		for _, c := range rest[:i] {
			version = version*10 + int(c-'0')
		}
	}

	rest = strings.TrimLeft(rest[i:], " \t")
	if !strings.HasPrefix(rest, `"`) {
		return Entry{}, &ParseError{Line: lineNo, Text: line, Msg: "value is not quoted"}
	}

	value, ok := unquote(rest)
	if !ok {
		// Recover the way the game's own parser does, by ending the value at
		// the line break, and let the caller decide what to do about it.
		value, _ = unquote(rest + `"`)
		return Entry{Key: key, Version: version, Value: value, Line: lineNo, Malformed: true},
			&ParseError{Line: lineNo, Text: line, Msg: "unterminated value (recovered at end of line)"}
	}

	return Entry{Key: key, Version: version, Value: value, Line: lineNo}, nil
}

// unquote reads a double-quoted string starting at s[0] and returns its
// contents. Only `\"` is resolved, because that escape is what delimits the
// string; every other backslash sequence is preserved verbatim so that values
// round trip byte for byte and so that `\n` survives as the two-character
// token the validator counts. Anything after the closing quote is ignored.
func unquote(s string) (string, bool) {
	var b strings.Builder
	for i := 1; i < len(s); i++ {
		switch s[i] {
		case '\\':
			if i+1 >= len(s) {
				b.WriteByte('\\')
				continue
			}
			i++
			if s[i] == '"' {
				b.WriteByte('"')
			} else {
				b.WriteByte('\\')
				b.WriteByte(s[i])
			}
		case '"':
			return b.String(), true
		default:
			b.WriteByte(s[i])
		}
	}
	return "", false
}

// Quote renders a value back into Paradox source form. It is the exact
// inverse of unquote: bare quotes are escaped, and backslash sequences that
// were preserved on the way in are emitted untouched.
func Quote(value string) string {
	var b strings.Builder
	b.WriteByte('"')
	for i := 0; i < len(value); i++ {
		switch c := value[i]; c {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			// Copy the escape and whatever it introduces as a unit, so that a
			// `\\` pair is never mistaken for an escape of the next byte.
			b.WriteByte('\\')
			if i+1 < len(value) {
				i++
				b.WriteByte(value[i])
			}
		default:
			b.WriteByte(c)
		}
	}
	b.WriteByte('"')
	return b.String()
}
