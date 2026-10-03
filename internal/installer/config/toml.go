package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/BurntSushi/toml"
)

// The bot's config is edited line by line, so the comments that document
// every setting survive; a TOML encoder would drop them.

var (
	header  = regexp.MustCompile(`^\[([^\[\]]+)\]\s*$`)
	keyLine = regexp.MustCompile(`^([A-Za-z0-9_-]+)\s*=`)
)

type section struct {
	name   string
	header string
	lines  []string
}

func parse(doc string) []*section {
	sections := []*section{{}}
	for line := range strings.SplitSeq(strings.TrimSuffix(doc, "\n"), "\n") {
		if m := header.FindStringSubmatch(line); m != nil {
			sections = append(sections, &section{name: strings.TrimSpace(m[1]), header: line})
			continue
		}
		last := sections[len(sections)-1]
		last.lines = append(last.lines, line)
	}
	return sections
}

func render(sections []*section) string {
	var b strings.Builder
	for _, s := range sections {
		if s.header != "" {
			b.WriteString(s.header + "\n")
		}
		for _, line := range s.lines {
			b.WriteString(line + "\n")
		}
	}
	return b.String()
}

func find(sections []*section, name string) *section {
	for _, s := range sections {
		if s.name == name {
			return s
		}
	}
	return nil
}

func (s *section) keys() []string {
	var keys []string
	for _, line := range s.lines {
		if m := keyLine.FindStringSubmatch(line); m != nil {
			keys = append(keys, m[1])
		}
	}
	return keys
}

// documented returns the key's line with the comments right above it.
func (s *section) documented(key string) []string {
	for i, line := range s.lines {
		if m := keyLine.FindStringSubmatch(line); m == nil || m[1] != key {
			continue
		}
		start := i
		for start > 0 && strings.HasPrefix(strings.TrimSpace(s.lines[start-1]), "#") {
			start--
		}
		return s.lines[start : i+1]
	}
	return nil
}

// body is the section's lines without the blank ones that close it.
func (s *section) body() []string {
	end := len(s.lines)
	for end > 0 && strings.TrimSpace(s.lines[end-1]) == "" {
		end--
	}
	return s.lines[:end]
}

// String is value as a TOML basic string.
func String(value string) string {
	var b bytes.Buffer
	encoder := json.NewEncoder(&b)
	encoder.SetEscapeHTML(false)
	_ = encoder.Encode(value)
	return strings.TrimSuffix(b.String(), "\n")
}

// Set replaces the line of key in table ("" is the top level) with value, a
// TOML literal.
func Set(doc, table, key, value string) (string, error) {
	sections := parse(doc)
	s := find(sections, table)
	found := 0
	if s != nil {
		for i, line := range s.lines {
			if m := keyLine.FindStringSubmatch(line); m != nil && m[1] == key {
				s.lines[i] = key + " = " + value
				found++
			}
		}
	}
	if found != 1 {
		return "", fmt.Errorf("config.toml must have exactly one %s", qualified(table, key))
	}
	return render(sections), nil
}

// Lookup reads key of table ("" is the top level).
func Lookup(doc, table, key string) (any, bool) {
	var values map[string]any
	if _, err := toml.Decode(doc, &values); err != nil {
		return nil, false
	}
	if table != "" {
		nested, ok := values[table].(map[string]any)
		if !ok {
			return nil, false
		}
		values = nested
	}
	value, ok := values[key]
	return value, ok
}

type Merged struct {
	Text string
	// Unknown are settings the example no longer has, as table.key.
	Unknown []string
}

// Merge adds to current the settings example has and current lacks, with
// their comments, and keeps everything current already says.
func Merge(current, example string) (Merged, error) {
	have := parse(current)
	for _, want := range parse(example) {
		target := find(have, want.name)
		if target == nil {
			if len(have[len(have)-1].body()) > 0 || len(have) > 1 {
				last := have[len(have)-1]
				last.lines = append(last.body(), "")
			}
			have = append(have, &section{name: want.name, header: want.header, lines: slices.Clone(want.body())})
			continue
		}
		for _, key := range want.keys() {
			if slices.Contains(target.keys(), key) {
				continue
			}
			body := target.body()
			rest := slices.Clone(target.lines[len(body):])
			target.lines = append(append(slices.Clone(body), want.documented(key)...), rest...)
		}
	}
	merged := Merged{Text: render(have)}
	if _, err := toml.Decode(merged.Text, new(map[string]any)); err != nil {
		return Merged{}, fmt.Errorf("merged config.toml is invalid: %w", err)
	}
	examples := parse(example)
	for _, s := range have {
		want := find(examples, s.name)
		for _, key := range s.keys() {
			if want == nil || !slices.Contains(want.keys(), key) {
				merged.Unknown = append(merged.Unknown, qualified(s.name, key))
			}
		}
	}
	return merged, nil
}

func qualified(table, key string) string {
	if table == "" {
		return key
	}
	return table + "." + key
}
