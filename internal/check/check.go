// Package check decides whether domain names are registered, using RDAP
// with a WHOIS fallback for TLDs that have no RDAP service.
package check

import (
	"context"
	"net/http"
	"sync"
	"time"
)

// Status is the outcome of a single lookup.
type Status string

const (
	StatusAvailable Status = "available"
	StatusTaken     Status = "taken"
	StatusUnknown   Status = "unknown"
)

// Result describes one domain lookup.
type Result struct {
	Domain    string `json:"domain"`
	Status    Status `json:"status"`
	Source    string `json:"source,omitempty"`
	Registrar string `json:"registrar,omitempty"`
	Created   string `json:"created,omitempty"`
	Expires   string `json:"expires,omitempty"`
	Error     string `json:"error,omitempty"`
}

func (r Result) fail(err error) Result {
	r.Status = StatusUnknown
	r.Error = err.Error()
	return r
}

// Checker runs lookups. Bootstrap and Whois must be set before use.
type Checker struct {
	HTTP      *http.Client
	Bootstrap *Bootstrap
	Whois     *Whois
	Timeout   time.Duration
}

// Check looks up a single normalized domain.
func (c *Checker) Check(ctx context.Context, domain string) Result {
	if c.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.Timeout)
		defer cancel()
	}
	if base, ok := c.Bootstrap.Lookup(domain); ok {
		return c.checkRDAP(ctx, base, domain)
	}
	return c.Whois.Check(ctx, domain)
}

// CheckAll looks up domains with at most concurrency requests in flight.
// Results are returned in input order.
func (c *Checker) CheckAll(ctx context.Context, domains []string, concurrency int) []Result {
	if concurrency < 1 {
		concurrency = 1
	}
	results := make([]Result, len(domains))
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	for i, d := range domains {
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			results[i] = c.Check(ctx, d)
		}()
	}
	wg.Wait()
	return results
}
