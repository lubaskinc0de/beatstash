package config

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"
)

// Env is a Compose .env file. Values are written double-quoted with
// backslashes, quotes and $ escaped, so Compose reads them back verbatim
// instead of interpolating them.
type Env struct {
	Path string
}

func (e Env) Read() (map[string]string, error) {
	values := map[string]string{}
	file, err := os.Open(e.Path)
	if errors.Is(err, os.ErrNotExist) {
		return values, nil
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		key, value, ok := strings.Cut(line, "=")
		if !ok || strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		values[strings.TrimSpace(key)] = decode(value)
	}
	return values, scanner.Err()
}

func (e Env) Get(key string) string {
	values, err := e.Read()
	if err != nil {
		return ""
	}
	return values[key]
}

// Set replaces key's line, or adds one, keeping the rest of the file.
func (e Env) Set(key, value string) error {
	if strings.ContainsAny(value, "\r\n") {
		return fmt.Errorf("%s must fit on one line", key)
	}
	data, err := os.ReadFile(e.Path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	line := key + "=" + encode(value)
	var lines []string
	replaced := false
	for existing := range strings.SplitSeq(strings.TrimSuffix(string(data), "\n"), "\n") {
		if name, _, ok := strings.Cut(existing, "="); ok && strings.TrimSpace(name) == key {
			if !replaced {
				lines = append(lines, line)
			}
			replaced = true
			continue
		}
		if existing != "" || len(data) > 0 {
			lines = append(lines, existing)
		}
	}
	if !replaced {
		lines = append(lines, line)
	}
	return WriteFile(e.Path, strings.Join(lines, "\n")+"\n")
}

func encode(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `"`, `\"`)
	value = strings.ReplaceAll(value, `$`, `$$`)
	return `"` + value + `"`
}

func decode(value string) string {
	value = strings.TrimSpace(value)
	if len(value) < 2 || value[0] != '"' || value[len(value)-1] != '"' {
		return value
	}
	value = value[1 : len(value)-1]
	var b strings.Builder
	for i := 0; i < len(value); i++ {
		if i+1 < len(value) && (value[i] == '\\' && (value[i+1] == '\\' || value[i+1] == '"') || value[i] == '$' && value[i+1] == '$') {
			i++
		}
		b.WriteByte(value[i])
	}
	return b.String()
}

// WriteFile rewrites path in place: a container that mounts a single file
// keeps the original inode and would never see a renamed replacement. A new
// file is readable by its owner only, as most of them hold secrets.
func WriteFile(path, content string) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600) //nolint:gosec // G304: files of the installation
	if err != nil {
		return err
	}
	if _, err := file.WriteString(content); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}
