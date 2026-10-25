package check

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

type rdapDomain struct {
	Events []struct {
		Action string `json:"eventAction"`
		Date   string `json:"eventDate"`
	} `json:"events"`
	Entities []rdapEntity `json:"entities"`
}

type rdapEntity struct {
	Roles      []string      `json:"roles"`
	VcardArray []interface{} `json:"vcardArray"`
	Entities   []rdapEntity  `json:"entities"`
}

const rdapAttempts = 6

// checkRDAP queries base for domain. 200 means registered, 404 means not.
// Rate-limited servers are retried, pacing all requests to the same base.
func (c *Checker) checkRDAP(ctx context.Context, base, domain string) Result {
	res := Result{Domain: domain, Source: "rdap"}
	url := base + "domain/" + domain
	gate := c.gates.get(base)
	retriedNet := false

	for attempt := 1; ; attempt++ {
		if err := gate.wait(ctx); err != nil {
			return res.fail(err)
		}
		resp, body, err := c.getRDAP(ctx, url)
		if err != nil {
			// Timeouts and resets are often transient; retry those once.
			if !retriedNet && ctx.Err() == nil {
				retriedNet = true
				if sleep(ctx, time.Second) == nil {
					continue
				}
			}
			return res.fail(err)
		}

		switch {
		case resp.StatusCode == http.StatusOK:
			res.Status = StatusTaken
			fillRDAPDetails(&res, body)
			return res
		case resp.StatusCode == http.StatusNotFound:
			res.Status = StatusAvailable
			return res
		case resp.StatusCode == http.StatusTooManyRequests && attempt < rdapAttempts:
			if every, slowed := gate.throttled(retryAfter(resp)); slowed && c.OnThrottle != nil {
				c.OnThrottle(base, every)
			}
		default:
			return res.fail(fmt.Errorf("rdap: HTTP %d from %s", resp.StatusCode, base))
		}
	}
}

// getRDAP performs one request. The timeout covers only the request itself,
// not time spent queued behind a throttled host.
func (c *Checker) getRDAP(ctx context.Context, url string) (*http.Response, []byte, error) {
	if c.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.Timeout)
		defer cancel()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("Accept", "application/rdap+json, application/json")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return nil, nil, err
	}
	return resp, body, nil
}

func fillRDAPDetails(res *Result, body []byte) {
	var d rdapDomain
	if json.Unmarshal(body, &d) != nil {
		return
	}
	for _, e := range d.Events {
		switch e.Action {
		case "registration":
			res.Created = dateOnly(e.Date)
		case "expiration":
			res.Expires = dateOnly(e.Date)
		}
	}
	res.Registrar = findRegistrar(d.Entities)
}

func findRegistrar(entities []rdapEntity) string {
	for _, e := range entities {
		for _, r := range e.Roles {
			if r == "registrar" {
				if fn := vcardFN(e.VcardArray); fn != "" {
					return fn
				}
			}
		}
		if fn := findRegistrar(e.Entities); fn != "" {
			return fn
		}
	}
	return ""
}

// vcardFN extracts the "fn" property from a jCard: ["vcard", [[name, params, type, value], ...]].
func vcardFN(v []interface{}) string {
	if len(v) < 2 {
		return ""
	}
	props, ok := v[1].([]interface{})
	if !ok {
		return ""
	}
	for _, p := range props {
		prop, ok := p.([]interface{})
		if !ok || len(prop) < 4 || prop[0] != "fn" {
			continue
		}
		if s, ok := prop[3].(string); ok {
			return s
		}
	}
	return ""
}

func dateOnly(s string) string {
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.UTC().Format("2006-01-02")
	}
	return s
}

// retryAfter returns the server's requested delay, or 0 when it gave none.
func retryAfter(resp *http.Response) time.Duration {
	if s, err := strconv.Atoi(resp.Header.Get("Retry-After")); err == nil && s > 0 && s <= 30 {
		return time.Duration(s) * time.Second
	}
	return 0
}

func sleep(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
