// Package i18n holds the Telegram adapter's texts: a YAML file per
// language, built-in English and Russian plus a directory from the config.
package i18n

import (
	"embed"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"

	goi18n "github.com/nicksnyder/go-i18n/v2/i18n"
	"go.yaml.in/yaml/v3"
	"golang.org/x/text/language"
)

// Language is a BCP 47 tag, as in the file name: "en", "pt-BR".
type Language string

const English Language = "en"

const (
	SourceURL = "https://github.com/lubaskinc0de/beatstash"
	Author    = "@lubaskinc0de"
)

//go:embed locales/*.yaml
var locales embed.FS

type Brand struct {
	Service string
	// NavidromeURL is where users open Navidrome; empty hides it.
	NavidromeURL string
}

type Options struct {
	// Dir: extra language files; empty means built-in only.
	Dir string
	// Default is for clients whose language has no file.
	Default Language
	Brand   Brand
}

type Bundle struct {
	bundle    *goi18n.Bundle
	languages []Language
	fallback  Language
	brand     Brand
}

// Load reads opts.Dir over the built-in files. Missing texts show in English.
func Load(opts Options) (*Bundle, error) {
	b := &Bundle{bundle: goi18n.NewBundle(language.English), fallback: opts.Default, brand: opts.Brand}
	b.bundle.RegisterUnmarshalFunc("yaml", yaml.Unmarshal)
	b.bundle.RegisterUnmarshalFunc("yml", yaml.Unmarshal)

	keys := map[Language]map[string]bool{}
	embedded, err := locales.ReadDir("locales")
	if err != nil {
		return nil, err
	}
	for _, entry := range embedded {
		data, err := locales.ReadFile("locales/" + entry.Name())
		if err != nil {
			return nil, err
		}
		if err := b.parse(data, entry.Name(), keys); err != nil {
			return nil, err
		}
	}
	if opts.Dir != "" {
		if err := b.loadDir(opts.Dir, keys); err != nil {
			return nil, err
		}
	}

	if b.fallback == "" {
		b.fallback = English
	}
	if keys[b.fallback] == nil {
		return nil, fmt.Errorf("no texts for the default language %q", b.fallback)
	}
	for lang := range keys {
		b.languages = append(b.languages, lang)
	}
	slices.Sort(b.languages)
	warnMissing(keys)
	return b, nil
}

func (b *Bundle) loadDir(dir string, keys map[Language]map[string]bool) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read translations: %w", err)
	}
	for _, entry := range entries {
		ext := filepath.Ext(entry.Name())
		if entry.IsDir() || ext != ".yaml" && ext != ".yml" {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(path) //nolint:gosec // G304: the directory comes from the config
		if err != nil {
			return err
		}
		if err := b.parse(data, path, keys); err != nil {
			return err
		}
	}
	return nil
}

func (b *Bundle) parse(data []byte, path string, keys map[Language]map[string]bool) error {
	file, err := b.bundle.ParseMessageFileBytes(data, path)
	if err != nil {
		return fmt.Errorf("translations %s: %w", path, err)
	}
	lang := Language(file.Tag.String())
	if keys[lang] == nil {
		keys[lang] = map[string]bool{}
	}
	for _, m := range file.Messages {
		keys[lang][m.ID] = true
	}
	return nil
}

func warnMissing(keys map[Language]map[string]bool) {
	for lang, have := range keys {
		var missing []string
		for key := range keys[English] {
			if !have[key] {
				missing = append(missing, key)
			}
		}
		if len(missing) > 0 {
			slices.Sort(missing)
			slog.Warn("translation_incomplete", "language", lang, "shown_in_english", strings.Join(missing, ", "))
		}
	}
}

func (b *Bundle) Languages() []Language {
	return b.languages
}

func (b *Bundle) Default() Language {
	return b.fallback
}

// Match tries the same tag, then the same base language, then the default.
func (b *Bundle) Match(code string) Language {
	if code == "" {
		return b.fallback
	}
	base, _, _ := strings.Cut(code, "-")
	for _, lang := range b.languages {
		if strings.EqualFold(string(lang), code) {
			return lang
		}
	}
	for _, lang := range b.languages {
		if l, _, _ := strings.Cut(string(lang), "-"); strings.EqualFold(l, base) {
			return lang
		}
	}
	return b.fallback
}

// For falls back to the default for a language without a file.
func (b *Bundle) For(lang Language) Catalog {
	if !slices.Contains(b.languages, lang) {
		lang = b.fallback
	}
	return Catalog{
		lang:  lang,
		loc:   goi18n.NewLocalizer(b.bundle, string(lang), string(b.fallback), string(English)),
		brand: b.brand,
	}
}
