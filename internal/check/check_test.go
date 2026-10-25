package check

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestNormalize(t *testing.T) {
	tests := []struct{ in, want string }{
		{"Example.COM", "example.com"},
		{"  example.com.  ", "example.com"},
		{"https://www.example.com/path?q=1", "example.com"},
		{"bücher.de", "xn--bcher-kva.de"},
	}
	for _, tt := range tests {
		got, err := Normalize(tt.in)
		if err != nil || got != tt.want {
			t.Errorf("Normalize(%q) = %q, %v; want %q", tt.in, got, err, tt.want)
		}
	}
	for _, bad := range []string{"", "bad domain", "a_b!.com"} {
		if _, err := Normalize(bad); err == nil {
			t.Errorf("Normalize(%q): expected error", bad)
		}
	}
}

func TestExpand(t *testing.T) {
	got, err := Expand([]string{"foo", "bar.io", "FOO.com"}, []string{"com", ".dev"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"foo.com", "foo.dev", "bar.io"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Expand = %v, want %v", got, want)
	}
}

const bootstrapJSON = `{"services":[
	[["com","net"],["http://insecure.example/","https://rdap.example/com"]],
	[["co.uk"],["https://rdap.example/couk/"]]
]}`

func TestBootstrapLookup(t *testing.T) {
	b, err := ParseBootstrap([]byte(bootstrapJSON))
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		domain, want string
		ok           bool
	}{
		{"foo.com", "https://rdap.example/com/", true},
		{"bbc.co.uk", "https://rdap.example/couk/", true},
		{"foo.de", "", false},
	}
	for _, tt := range tests {
		got, ok := b.Lookup(tt.domain)
		if got != tt.want || ok != tt.ok {
			t.Errorf("Lookup(%q) = %q, %v; want %q, %v", tt.domain, got, ok, tt.want, tt.ok)
		}
	}
}

func TestLoadBootstrapCache(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		fmt.Fprint(w, bootstrapJSON)
	}))
	defer srv.Close()

	cache := filepath.Join(t.TempDir(), "sub", "dns.json")
	ctx := context.Background()
	for i := 0; i < 2; i++ {
		if _, err := LoadBootstrap(ctx, srv.Client(), srv.URL, cache, time.Hour); err != nil {
			t.Fatal(err)
		}
	}
	if hits != 1 {
		t.Errorf("expected 1 download with a fresh cache, got %d", hits)
	}

	// A stale cache is still used when the download fails.
	old := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(cache, old, old); err != nil {
		t.Fatal(err)
	}
	srv.Close()
	if _, err := LoadBootstrap(ctx, http.DefaultClient, srv.URL, cache, time.Hour); err != nil {
		t.Errorf("expected stale cache fallback, got %v", err)
	}
}

const takenRDAP = `{
	"events": [
		{"eventAction": "registration", "eventDate": "1997-09-15T04:00:00Z"},
		{"eventAction": "expiration", "eventDate": "2028-09-14T04:00:00Z"}
	],
	"entities": [{
		"roles": ["registrar"],
		"vcardArray": ["vcard", [["version", {}, "text", "4.0"], ["fn", {}, "text", "Example Registrar"]]]
	}]
}`

func newRDAPChecker(t *testing.T, handler http.HandlerFunc) *Checker {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	b, err := ParseBootstrap([]byte(`{"services":[[["com"],["` + srv.URL + `/"]]]}`))
	if err != nil {
		t.Fatal(err)
	}
	return &Checker{HTTP: srv.Client(), Bootstrap: b, Timeout: 5 * time.Second}
}

func TestCheckRDAP(t *testing.T) {
	var limited int32
	c := newRDAPChecker(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/domain/taken.com":
			fmt.Fprint(w, takenRDAP)
		case "/domain/free.com":
			http.NotFound(w, r)
		case "/domain/limited.com":
			if atomic.AddInt32(&limited, 1) == 1 {
				w.Header().Set("Retry-After", "1")
				w.WriteHeader(http.StatusTooManyRequests)
				return
			}
			http.NotFound(w, r)
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}
	})

	results := c.CheckAll(context.Background(), []string{"taken.com", "free.com", "limited.com", "broken.com"}, 4)
	want := []Result{
		{Domain: "taken.com", Status: StatusTaken, Source: "rdap", Registrar: "Example Registrar", Created: "1997-09-15", Expires: "2028-09-14"},
		{Domain: "free.com", Status: StatusAvailable, Source: "rdap"},
		{Domain: "limited.com", Status: StatusAvailable, Source: "rdap"},
	}
	if !reflect.DeepEqual(results[:3], want) {
		t.Errorf("results = %+v\nwant %+v", results[:3], want)
	}
	if r := results[3]; r.Status != StatusUnknown || !strings.Contains(r.Error, "HTTP 500") {
		t.Errorf("broken.com = %+v, want unknown with HTTP 500", r)
	}
}

func TestClassifyWhois(t *testing.T) {
	disclaimer := strings.Repeat("Terms of use apply to this record.\n", whoisHeadLines) +
		"Queries are not found to be abusive unless too many are sent.\n"
	tests := []struct {
		name string
		resp string
		want Status
	}{
		{"io free", "Domain not found.\n>>> Last update of WHOIS database <<<\n", StatusAvailable},
		{"de free", "Domain: xq7zv9kpl2wm.de\nStatus: free\n", StatusAvailable},
		{"de taken", "Domain: google.de\nStatus: connect\n", StatusTaken},
		{"io taken with boilerplate", "This domain is protected by the Registry Lock service.\n\nDomain Name: google.io\n" + disclaimer, StatusTaken},
		{"rate limited", "Query rate limit exceeded. Try again later.\n", StatusUnknown},
		{"empty", "\r\n\r\n", StatusUnknown},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := classifyWhois(tt.resp)
			if got != tt.want {
				t.Errorf("classifyWhois = %s (err %v), want %s", got, err, tt.want)
			}
			if (got == StatusUnknown) != (err != nil) {
				t.Errorf("unknown status must come with an error, got err %v", err)
			}
		})
	}
}

// fakeWhois serves canned answers keyed by the query line.
func fakeWhois(t *testing.T, answers map[string]string) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()
				q, _ := bufio.NewReader(conn).ReadString('\n')
				fmt.Fprint(conn, answers[strings.TrimSpace(q)])
			}()
		}
	}()
	return ln.Addr().String()
}

func TestWhoisFallback(t *testing.T) {
	var registry string
	iana := fakeWhois(t, map[string]string{"de": "domain: DE\nwhois: REGISTRY\n"})
	registry = fakeWhois(t, map[string]string{
		"google.de": "Domain: google.de\nStatus: connect\n",
		"free.de":   "Domain: free.de\nStatus: free\n",
	})

	w := NewWhois(time.Second)
	w.IANA = iana
	dial := w.Dial
	w.Dial = func(ctx context.Context, addr string) (net.Conn, error) {
		if addr == "REGISTRY:43" {
			addr = registry
		}
		return dial(ctx, addr)
	}

	b, _ := ParseBootstrap([]byte(bootstrapJSON))
	c := &Checker{Bootstrap: b, Whois: w, Timeout: 5 * time.Second}
	got := c.CheckAll(context.Background(), []string{"google.de", "free.de"}, 2)
	want := []Result{
		{Domain: "google.de", Status: StatusTaken, Source: "whois"},
		{Domain: "free.de", Status: StatusAvailable, Source: "whois"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}

	if r := c.Check(context.Background(), "foo.zz"); r.Status != StatusUnknown || !strings.Contains(r.Error, "no RDAP or WHOIS server") {
		t.Errorf("unknown TLD = %+v", r)
	}
}
