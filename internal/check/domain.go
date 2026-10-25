package check

import (
	"fmt"
	"strings"

	"golang.org/x/net/idna"
)

// Normalize turns user input into a lowercase ASCII (punycode) domain name.
// It tolerates pasted URLs ("https://foo.com/path") and trailing dots.
func Normalize(s string) (string, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.TrimPrefix(s, "http://")
	s = strings.TrimPrefix(s, "https://")
	if i := strings.IndexAny(s, "/?#"); i >= 0 {
		s = s[:i]
	}
	s = strings.TrimPrefix(s, "www.")
	s = strings.TrimSuffix(s, ".")
	if s == "" {
		return "", fmt.Errorf("empty domain")
	}
	ascii, err := idna.Lookup.ToASCII(s)
	if err != nil {
		return "", fmt.Errorf("invalid domain %q: %w", s, err)
	}
	return ascii, nil
}

// Expand normalizes inputs and turns bare names (no dot) into one domain per TLD.
// Duplicates are dropped while keeping first-seen order.
func Expand(inputs, tlds []string) ([]string, error) {
	seen := make(map[string]bool)
	var out []string
	add := func(d string) {
		if !seen[d] {
			seen[d] = true
			out = append(out, d)
		}
	}
	for _, in := range inputs {
		name, err := Normalize(in)
		if err != nil {
			return nil, err
		}
		if strings.Contains(name, ".") {
			add(name)
			continue
		}
		for _, tld := range tlds {
			d, err := Normalize(name + "." + strings.TrimPrefix(tld, "."))
			if err != nil {
				return nil, err
			}
			add(d)
		}
	}
	return out, nil
}
