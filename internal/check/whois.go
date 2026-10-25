package check

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"time"
)

// IANAWhois answers "which WHOIS server handles this TLD?".
const IANAWhois = "whois.iana.org:43"

// Phrases registries use to say a name is not registered.
var whoisAvailable = []string{
	"no match",
	"not found",
	"no entries found",
	"no data found",
	"no object found",
	"nothing found",
	"status: free",
	"status: available",
	"is available for",
	"is free",
	"no matching record",
	"domain not registered",
	"not registered",
	"does not exist",
}

// Phrases that mean the server refused to answer, so we know nothing.
var whoisRefused = []string{
	"limit exceeded",
	"query rate",
	"too many",
	"try again later",
	"access denied",
	"blacklisted",
}

// Whois is a minimal port-43 client that discovers TLD servers through IANA.
type Whois struct {
	Dial func(ctx context.Context, addr string) (net.Conn, error)
	IANA string

	mu      sync.Mutex
	servers map[string]string
}

// NewWhois returns a Whois client using a standard TCP dialer.
func NewWhois(timeout time.Duration) *Whois {
	d := &net.Dialer{Timeout: timeout}
	return &Whois{
		Dial: func(ctx context.Context, addr string) (net.Conn, error) {
			return d.DialContext(ctx, "tcp", addr)
		},
		IANA: IANAWhois,
	}
}

func (w *Whois) query(ctx context.Context, addr, q string) (string, error) {
	conn, err := w.Dial(ctx, addr)
	if err != nil {
		return "", err
	}
	defer conn.Close()
	if dl, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(dl)
	}
	if _, err := fmt.Fprintf(conn, "%s\r\n", q); err != nil {
		return "", err
	}
	data, err := io.ReadAll(io.LimitReader(conn, 1<<20))
	if err != nil && len(data) == 0 {
		return "", err
	}
	return string(data), nil
}

// serverFor asks IANA which WHOIS server handles tld and caches the answer.
func (w *Whois) serverFor(ctx context.Context, tld string) (string, error) {
	w.mu.Lock()
	if s, ok := w.servers[tld]; ok {
		w.mu.Unlock()
		return s, nil
	}
	w.mu.Unlock()

	resp, err := w.query(ctx, w.IANA, tld)
	if err != nil {
		return "", fmt.Errorf("whois: iana lookup for .%s: %w", tld, err)
	}
	server := ""
	sc := bufio.NewScanner(strings.NewReader(resp))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if v, ok := strings.CutPrefix(line, "whois:"); ok {
			server = strings.TrimSpace(v)
			break
		}
	}
	if server == "" {
		return "", fmt.Errorf("no RDAP or WHOIS server known for .%s", tld)
	}
	if !strings.Contains(server, ":") {
		server += ":43"
	}

	w.mu.Lock()
	if w.servers == nil {
		w.servers = make(map[string]string)
	}
	w.servers[tld] = server
	w.mu.Unlock()
	return server, nil
}

// Check looks domain up on its TLD's WHOIS server.
func (w *Whois) Check(ctx context.Context, domain string) Result {
	res := Result{Domain: domain, Source: "whois"}
	tld := domain[strings.LastIndex(domain, ".")+1:]
	server, err := w.serverFor(ctx, tld)
	if err != nil {
		return res.fail(err)
	}
	resp, err := w.query(ctx, server, domain)
	if err != nil {
		// Some registries reset connections under bursts; one retry is usually enough.
		if sleep(ctx, time.Second) == nil {
			resp, err = w.query(ctx, server, domain)
		}
	}
	if err != nil {
		return res.fail(fmt.Errorf("whois %s: %w", server, err))
	}
	res.Status, err = classifyWhois(resp)
	if err != nil {
		return res.fail(fmt.Errorf("whois %s: %w", server, err))
	}
	return res
}

// whoisHeadLines bounds how much of a reply is searched for verdict phrases.
// Registries put the verdict at the top; the legal boilerplate further down
// often mentions phrases like "not found" or "too many queries".
const whoisHeadLines = 15

func classifyWhois(resp string) (Status, error) {
	var head []string
	for _, line := range strings.Split(strings.ToLower(resp), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			head = append(head, line)
			if len(head) == whoisHeadLines {
				break
			}
		}
	}
	if len(head) == 0 {
		return StatusUnknown, fmt.Errorf("empty response")
	}
	text := strings.Join(head, "\n")
	for _, p := range whoisAvailable {
		if strings.Contains(text, p) {
			return StatusAvailable, nil
		}
	}
	for _, line := range head {
		if strings.HasPrefix(line, "domain name:") || strings.HasPrefix(line, "domain:") {
			return StatusTaken, nil
		}
	}
	for _, p := range whoisRefused {
		if strings.Contains(text, p) {
			return StatusUnknown, fmt.Errorf("server refused query (%s)", p)
		}
	}
	return StatusTaken, nil
}
