package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/berkaycubuk/isdomaintaken/internal/check"
	"github.com/berkaycubuk/isdomaintaken/internal/cli"
)

// setup points the command at a fake RDAP registry for .com where only
// taken.com is registered and broken.com errors.
func setup(t *testing.T) {
	t.Helper()
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	mux.HandleFunc("/dns.json", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"services":[[["com"],["%s/rdap/"]]]}`, srv.URL)
	})
	mux.HandleFunc("/rdap/domain/", func(w http.ResponseWriter, r *http.Request) {
		switch strings.TrimPrefix(r.URL.Path, "/rdap/domain/") {
		case "taken.com":
			fmt.Fprint(w, `{"events":[{"eventAction":"expiration","eventDate":"2030-01-02T00:00:00Z"}]}`)
		case "broken.com":
			w.WriteHeader(http.StatusBadGateway)
		default:
			http.NotFound(w, r)
		}
	})

	oldURL, oldCache, oldWhois := bootstrapURL, cacheFile, newWhois
	bootstrapURL = srv.URL + "/dns.json"
	cache := filepath.Join(t.TempDir(), "dns.json")
	cacheFile = func() string { return cache }
	newWhois = func(time.Duration) *check.Whois {
		w := check.NewWhois(time.Second)
		w.IANA = "127.0.0.1:1" // nothing listens here
		return w
	}
	t.Cleanup(func() { bootstrapURL, cacheFile, newWhois = oldURL, oldCache, oldWhois })
}

func exitCode(err error) cli.ExitCode {
	if err == nil {
		return cli.CodeSuccess
	}
	var ce *cli.Error
	if errors.As(err, &ce) {
		return ce.Code
	}
	return cli.CodeError
}

func TestRootPlain(t *testing.T) {
	setup(t)
	stdout, _, err := cli.ExecuteCommand(newRootCmd(), "free", "taken.com", "-t", "com")
	if exitCode(err) != CodeTaken {
		t.Fatalf("exit = %d (%v), want %d", exitCode(err), err, CodeTaken)
	}
	for _, want := range []string{"free.com", "available", "taken.com", "taken", "2030-01-02"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout missing %q:\n%s", want, stdout)
		}
	}
}

func TestRootJSONAvailableOnlyFromStdin(t *testing.T) {
	setup(t)
	cmd := newRootCmd()
	cmd.SetIn(strings.NewReader("# ideas\nfree\n\ntaken another\n"))
	stdout, _, err := cli.ExecuteCommand(cmd, "-o", "json", "--available")
	if exitCode(err) != CodeTaken {
		t.Fatalf("exit = %d (%v), want %d", exitCode(err), err, CodeTaken)
	}
	var got []check.Result
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout)
	}
	if len(got) != 2 || got[0].Domain != "free.com" || got[1].Domain != "another.com" {
		t.Errorf("got %+v, want free.com and another.com", got)
	}
}

func TestRootAllAvailable(t *testing.T) {
	setup(t)
	if _, _, err := cli.ExecuteCommand(newRootCmd(), "free.com"); err != nil {
		t.Errorf("expected success, got %v", err)
	}
}

func TestRootLookupFailure(t *testing.T) {
	setup(t)
	_, _, err := cli.ExecuteCommand(newRootCmd(), "broken.com", "taken.com", "-q")
	if exitCode(err) != cli.CodeFatal {
		t.Errorf("exit = %d (%v), want %d", exitCode(err), err, cli.CodeFatal)
	}
}

func TestRootBadInput(t *testing.T) {
	setup(t)
	_, _, err := cli.ExecuteCommand(newRootCmd(), "not a domain")
	if exitCode(err) != cli.CodeError {
		t.Errorf("exit = %d (%v), want %d", exitCode(err), err, cli.CodeError)
	}
}
