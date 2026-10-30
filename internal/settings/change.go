package settings

import (
	"bytes"
	"errors"
	"os"
	"slices"
)

// Change applies one edit to the freshest settings and saves it at once - a healthy file only has the
// changed keys rewritten so its comments and hand edits stay - a file that can't be edited in place is
// replaced whole and the old one kept - backup names that copy
func (f *File) Change(edit func(*Settings)) (backup string, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.refresh()
	next := f.current
	next.Columns, next.Modes = slices.Clone(next.Columns), slices.Clone(next.Modes)
	edit(&next)
	var changed []field
	for _, fd := range fields {
		if fd.value(next) != fd.value(f.current) {
			changed = append(changed, fd)
		}
	}
	if len(changed) == 0 {
		return "", nil
	}
	raw, err := os.ReadFile(f.path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	data, inPlace := []byte(nil), false
	if !f.invalid {
		data, inPlace = editedInPlace(raw, changed, next)
	}
	if !inPlace {
		data = next.contents()
	}
	s, note, err := Parse(data)
	if err != nil {
		return "", err
	}
	if !inPlace && len(bytes.TrimSpace(raw)) > 0 {
		backup = FileName + ".bak"
		if err := os.WriteFile(f.path+".bak", raw, 0o644); err != nil {
			return "", err
		}
	}
	if err := replace(f.path, data); err != nil {
		return "", err
	}
	f.current, f.warning, f.invalid = s, "", false
	if note != "" {
		f.warning = FileName + ": " + note
	}
	f.exists = true
	if fi, err := os.Stat(f.path); err == nil {
		f.mtime = fi.ModTime()
	}
	return backup, nil
}

// replace swaps the whole file in one step so a crash can't leave half a settings file
func replace(path string, data []byte) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}

// editedInPlace is false when the file has nothing to keep or the edit didn't read back as asked
func editedInPlace(raw []byte, changed []field, next Settings) ([]byte, bool) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil, false
	}
	data := raw
	for _, fd := range changed {
		data = setKey(data, fd, fd.value(next))
	}
	parsed, _, err := Parse(data)
	if err != nil {
		return nil, false
	}
	for _, fd := range changed {
		if fd.value(parsed) != fd.value(next) {
			return nil, false
		}
	}
	return data, true
}

// setKey swaps a key's value - a key the file doesn't have yet goes at the end with its comment
func setKey(data []byte, fd field, value string) []byte {
	if start, end, ok := findValue(data, fd.key); ok {
		return slices.Concat(data[:start], []byte(value), data[end:])
	}
	out := bytes.Clone(data)
	if !bytes.HasSuffix(out, []byte("\n")) {
		out = append(out, '\n')
	}
	return append(append(out, '\n'), fd.block(value)...)
}

var utf8BOM = []byte("\xef\xbb\xbf")

// findValue scans the file's top-level keys for where a value sits - the toml library doesn't say
func findValue(data []byte, key string) (start, end int, ok bool) {
	i := len(data) - len(bytes.TrimPrefix(data, utf8BOM))
	for {
		i = skipBlank(data, i)
		if i >= len(data) {
			return 0, 0, false
		}
		name, next := scanKey(data, i)
		next = skipSpaces(data, next)
		if next >= len(data) || data[next] != '=' {
			return 0, 0, false
		}
		start = skipSpaces(data, next+1)
		end = scanValue(data, start)
		if name == key {
			return start, end, true
		}
		i = end
	}
}

func skipSpaces(data []byte, i int) int {
	for i < len(data) && (data[i] == ' ' || data[i] == '\t') {
		i++
	}
	return i
}

func skipBlank(data []byte, i int) int {
	for i < len(data) {
		switch data[i] {
		case ' ', '\t', '\r', '\n':
			i++
		case '#':
			i = lineEnd(data, i)
		default:
			return i
		}
	}
	return i
}

func lineEnd(data []byte, i int) int {
	if n := bytes.IndexByte(data[i:], '\n'); n >= 0 {
		return i + n
	}
	return len(data)
}

func scanKey(data []byte, i int) (string, int) {
	if data[i] == '"' || data[i] == '\'' {
		end := scanString(data, i)
		return string(data[i+1 : max(i+1, end-1)]), end
	}
	start := i
	for i < len(data) && !bytes.ContainsAny(data[i:i+1], " \t=\r\n") {
		i++
	}
	return string(data[start:i]), i
}

func scanValue(data []byte, i int) int {
	if i >= len(data) {
		return i
	}
	switch data[i] {
	case '"', '\'':
		return scanString(data, i)
	case '[':
		return scanArray(data, i)
	}
	for i < len(data) && !bytes.ContainsAny(data[i:i+1], " \t\r\n#") {
		i++
	}
	return i
}

// scanArray can span lines and hold comments and strings with brackets in them
func scanArray(data []byte, i int) int {
	depth := 0
	for i < len(data) {
		switch data[i] {
		case '[':
			depth++
			i++
		case ']':
			depth--
			i++
			if depth == 0 {
				return i
			}
		case '#':
			i = lineEnd(data, i)
		case '"', '\'':
			i = scanString(data, i)
		default:
			i++
		}
	}
	return i
}

func scanString(data []byte, i int) int {
	quote := data[i]
	delim := data[i : i+1]
	if triple := bytes.Repeat(delim, 3); bytes.HasPrefix(data[i:], triple) {
		delim = triple
	}
	for j := i + len(delim); j < len(data); j++ {
		if quote == '"' && data[j] == '\\' {
			j++
			continue
		}
		if bytes.HasPrefix(data[j:], delim) {
			return j + len(delim)
		}
	}
	return len(data)
}
