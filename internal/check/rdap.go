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

const rdapAttempts = 3

// checkRDAP queries base for domain. 200 means registered, 404 means not.
func (c *Checker) checkRDAP(ctx context.Context, base, domain string) Result {
	res := Result{Domain: domain, Source: "rdap"}
	url := base + "domain/" + domain

	for attempt := 1; ; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return res.fail(err)
		}
		req.Header.Set("Accept", "application/rdap+json, application/json")
		resp, err := c.HTTP.Do(req)
		if err != nil {
			return res.fail(err)
		}
		body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
		resp.Body.Close()
		if err != nil {
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
			if err := sleep(ctx, retryAfter(resp, attempt)); err != nil {
				return res.fail(err)
			}
		default:
			return res.fail(fmt.Errorf("rdap: HTTP %d from %s", resp.StatusCode, base))
		}
	}
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

func retryAfter(resp *http.Response, attempt int) time.Duration {
	if s, err := strconv.Atoi(resp.Header.Get("Retry-After")); err == nil && s > 0 && s <= 30 {
		return time.Duration(s) * time.Second
	}
	return time.Duration(attempt) * time.Second
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
