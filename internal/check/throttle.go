package check

import (
	"context"
	"sync"
	"time"
)

const (
	throttleStart = time.Second
	throttleMax   = 8 * time.Second
)

// hostGate paces requests to one server. It is wide open until the server
// rate-limits us, then spaces requests out, backing off further on each 429.
type hostGate struct {
	mu       sync.Mutex
	next     time.Time     // earliest start for the next request
	interval time.Duration // spacing between requests; 0 until throttled
	last     time.Time     // when interval was last raised
}

// wait blocks until this caller may send its request.
func (g *hostGate) wait(ctx context.Context) error {
	g.mu.Lock()
	start := time.Now()
	if g.next.After(start) {
		start = g.next
	}
	g.next = start.Add(g.interval)
	g.mu.Unlock()
	return sleep(ctx, time.Until(start))
}

// throttled records a 429 and pushes back the next request by at least retryAfter.
// It reports the request spacing and whether this call slowed it down further.
func (g *hostGate) throttled(retryAfter time.Duration) (time.Duration, bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	now := time.Now()
	// 429s from requests that were already in flight when we last backed off
	// say nothing new about the pacing, so only escalate once per interval.
	slowed := true
	switch {
	case g.interval == 0:
		g.interval, g.last = throttleStart, now
	case g.interval < throttleMax && now.Sub(g.last) >= g.interval:
		g.interval, g.last = g.interval*2, now
	default:
		slowed = false
	}
	until := now.Add(max(retryAfter, g.interval))
	if g.next.Before(until) {
		g.next = until
	}
	return g.interval, slowed
}

// gates hands out one hostGate per server.
type gates struct {
	mu sync.Mutex
	m  map[string]*hostGate
}

func (gs *gates) get(host string) *hostGate {
	gs.mu.Lock()
	defer gs.mu.Unlock()
	if gs.m == nil {
		gs.m = make(map[string]*hostGate)
	}
	g, ok := gs.m[host]
	if !ok {
		g = &hostGate{}
		gs.m[host] = g
	}
	return g
}
