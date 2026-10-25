package check

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// IANABootstrapURL lists the RDAP server responsible for each TLD (RFC 9224).
const IANABootstrapURL = "https://data.iana.org/rdap/dns.json"

// Bootstrap maps TLDs to their RDAP base URL.
type Bootstrap struct {
	services map[string]string
}

type bootstrapFile struct {
	Services [][][]string `json:"services"`
}

// ParseBootstrap parses an IANA RDAP DNS bootstrap file.
func ParseBootstrap(data []byte) (*Bootstrap, error) {
	var f bootstrapFile
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("parse bootstrap: %w", err)
	}
	b := &Bootstrap{services: make(map[string]string)}
	for _, svc := range f.Services {
		if len(svc) < 2 || len(svc[1]) == 0 {
			continue
		}
		base := pickURL(svc[1])
		for _, tld := range svc[0] {
			b.services[strings.ToLower(tld)] = base
		}
	}
	if len(b.services) == 0 {
		return nil, fmt.Errorf("parse bootstrap: no services found")
	}
	return b, nil
}

// pickURL prefers https and makes sure the base ends with a slash.
func pickURL(urls []string) string {
	u := urls[0]
	for _, c := range urls {
		if strings.HasPrefix(c, "https://") {
			u = c
			break
		}
	}
	if !strings.HasSuffix(u, "/") {
		u += "/"
	}
	return u
}

// Lookup returns the RDAP base URL for the longest matching suffix of domain.
func (b *Bootstrap) Lookup(domain string) (base string, ok bool) {
	labels := strings.Split(domain, ".")
	for i := 1; i < len(labels); i++ {
		if u, found := b.services[strings.Join(labels[i:], ".")]; found {
			return u, true
		}
	}
	return "", false
}

// LoadBootstrap returns the bootstrap file from cachePath when it is younger than
// maxAge, and downloads it otherwise. A stale cache is used if the download fails.
func LoadBootstrap(ctx context.Context, client *http.Client, url, cachePath string, maxAge time.Duration) (*Bootstrap, error) {
	var cached []byte
	if cachePath != "" {
		if st, err := os.Stat(cachePath); err == nil {
			if data, err := os.ReadFile(cachePath); err == nil {
				cached = data
				if time.Since(st.ModTime()) < maxAge {
					if b, err := ParseBootstrap(data); err == nil {
						return b, nil
					}
				}
			}
		}
	}

	data, err := fetchBootstrap(ctx, client, url)
	if err == nil {
		b, perr := ParseBootstrap(data)
		if perr == nil {
			if cachePath != "" {
				// Cache failures are not worth failing the run over.
				if os.MkdirAll(filepath.Dir(cachePath), 0o755) == nil {
					_ = os.WriteFile(cachePath, data, 0o644)
				}
			}
			return b, nil
		}
		err = perr
	}
	if cached != nil {
		if b, perr := ParseBootstrap(cached); perr == nil {
			return b, nil
		}
	}
	return nil, err
}

func fetchBootstrap(ctx context.Context, client *http.Client, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch bootstrap: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch bootstrap: HTTP %d", resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 10<<20))
}
