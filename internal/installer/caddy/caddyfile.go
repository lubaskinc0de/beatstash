package caddy

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

const (
	begin = "# beatstash:begin"
	end   = "# beatstash:end"
)

// ErrForeignSite means the Caddyfile already serves the domain outside the
// beatstash block, so adding ours would clash with or take over that site.
var ErrForeignSite = errors.New("the Caddyfile already serves this domain")

// Site is the Caddyfile site that proxies domain to upstream.
func Site(domain, upstream string) string {
	return fmt.Sprintf("%s {\n\treverse_proxy %s\n}\n", domain, upstream)
}

func block(domain, upstream string) string {
	return begin + "\n" + Site(domain, upstream) + end + "\n"
}

// WithSite puts the beatstash block for domain in caddyfile: in place of the
// previous one, else at the end. Every other line stays as it was.
func WithSite(caddyfile, domain, upstream string) (string, error) {
	if serves(WithoutSite(caddyfile), domain) {
		return "", ErrForeignSite
	}
	if start, stop, ok := markers(caddyfile); ok {
		return caddyfile[:start] + block(domain, upstream) + strings.TrimPrefix(caddyfile[stop:], "\n"), nil
	}
	if caddyfile != "" {
		caddyfile = strings.TrimRight(caddyfile, "\n") + "\n\n"
	}
	return caddyfile + block(domain, upstream), nil
}

// markers locates the beatstash block, from its first byte to just past
// the end marker.
func markers(caddyfile string) (start, stop int, ok bool) {
	start = strings.Index(caddyfile, begin)
	if start < 0 {
		return 0, 0, false
	}
	length := strings.Index(caddyfile[start:], end)
	if length < 0 {
		return 0, 0, false
	}
	return start, start + length + len(end), true
}

// WithoutSite removes the beatstash block and the blank line before it.
func WithoutSite(caddyfile string) string {
	start, stop, ok := markers(caddyfile)
	if !ok {
		return caddyfile
	}
	before := strings.TrimRight(caddyfile[:start], "\n")
	after := strings.TrimLeft(caddyfile[stop:], "\n")
	switch {
	case before == "":
		return after
	case after == "":
		return before + "\n"
	default:
		return before + "\n\n" + after
	}
}

func serves(caddyfile, domain string) bool {
	address := regexp.MustCompile(`(^|[\s,/])` + regexp.QuoteMeta(domain) + `($|[\s,:{])`)
	for line := range strings.SplitSeq(caddyfile, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "#") && address.MatchString(line) {
			return true
		}
	}
	return false
}
